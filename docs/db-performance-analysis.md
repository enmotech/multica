# Database Performance Analysis

> Reviewed: 2026-05-27  
> Branch: main (upstream multica-ai/multica, commit 5f1f08e4)   
> Scope: Go backend + PostgreSQL (sqlc queries, service layer, migrations)  
> Status: All 9 findings confirmed still present on main as of this date

This document identifies the main database performance bottlenecks under high concurrency — specifically many agents executing tasks in parallel, many users chatting with agents simultaneously. Findings are ordered by severity.

Each finding includes a **Scale Threshold** estimate — the approximate scale at which the issue transitions from theoretical to user-visible. These are rough order-of-magnitude guides based on query structure and index coverage, not benchmarked numbers.

**Quick reference — "when should I worry?"**

| Workspace profile | Agents | Issues | Tasks/day | Likely pain points |
|---|---|---|---|---|
| Small team (self-hosted) | 2–5 | < 500 | < 50 | None — all findings are theoretical at this scale |
| Medium team | 5–15 | 500–2k | 50–200 | #7 (inbox index) after 3–6 months of accumulation |
| Large team | 15–50 | 2k–10k | 200–1000 | #1 (snapshot scan), #2 (reconcile bursts), #4 (resolve overhead) |
| High-throughput / enterprise | 50+ | 10k+ | 1000+ | All findings become relevant; #1 and #3 are blocking |

---

## CRITICAL

### 1. `ListWorkspaceAgentTaskSnapshot` — workspace-scoped full scan with no `workspace_id` column on the queue table

**Query** (`server/pkg/db/queries/agent.sql`):

```sql
-- Active tasks half
SELECT atq.* FROM agent_task_queue atq
JOIN agent a ON a.id = atq.agent_id
WHERE a.workspace_id = $1
  AND atq.status IN ('queued', 'dispatched', 'running')

UNION ALL

-- Latest outcome half (DISTINCT ON)
SELECT t.* FROM (
  SELECT DISTINCT ON (atq.agent_id) atq.*
  FROM agent_task_queue atq
  JOIN agent a ON a.id = atq.agent_id
  WHERE a.workspace_id = $1
    AND atq.status IN ('completed', 'failed')
  ORDER BY atq.agent_id, atq.completed_at DESC NULLS LAST
) t;
```

**Problem:**  
`agent_task_queue` has no `workspace_id` column. Every workspace-scoped query must JOIN through `agent` to filter by workspace. Under high concurrency:

- The **active tasks half** scans `agent_task_queue` for all `queued/dispatched/running` rows, then joins to `agent` to filter by workspace — potentially cross-workspace scans before the JOIN filter applies.
- The **outcome half** runs `DISTINCT ON (agent_id)` over all `completed/failed` rows in the workspace. As historical task rows accumulate (completed tasks are never purged), this scan grows unboundedly.
- There is no compound index on `(agent_id, workspace_id)` or `(workspace_id, status, completed_at)` that would let the planner skip irrelevant workspaces without the JOIN.

**Impact scope:**  
Every frontend surface that shows agent real-time state calls this endpoint:
- Agents list page (presence indicators, working/idle badge)
- Dojo page (room positioning and animations)
- WebSocket reconnect re-hydration (client calls this to restore state after disconnect)
- Any component using `agentTaskSnapshotOptions` from TanStack Query

This query runs per-workspace, per-reconnect, and on any poll cycle. A workspace with 20 agents accumulating 100k+ historical task rows will see this degrade linearly.

**Scale Threshold:**
- **Agents:** 10+ per workspace
- **Historical tasks:** 50k+ rows in `agent_task_queue` (roughly: 10 agents × 20 tasks/day × 250 working days = 50k in ~1 year without purging)
- **Concurrent viewers:** 5+ users with the Agents page open (each triggers this query on WS reconnect and staleTime refresh)
- **Symptom onset:** query time exceeds 100ms when the outcome half scans 50k+ completed/failed rows to find the latest per agent. At 200k+ rows, expect 300–500ms per call.

**Recommendation:**
1. Add `workspace_id UUID NOT NULL` to `agent_task_queue` (denormalize for query efficiency; keep the FK to `agent` for integrity). This eliminates the workspace-scoping JOIN.
2. For the outcome half: add a partial index `ON agent_task_queue (agent_id, completed_at DESC) WHERE status IN ('completed', 'failed')` to make the `DISTINCT ON` bounded.
3. Longer term: maintain a `agent_presence_snapshot` materialized table updated by triggers or a background job, so the read path is a trivial primary key lookup.

---

## HIGH

### 2. `ReconcileAgentStatus` — write hotspot on every task lifecycle event

**Service layer** (`server/internal/service/task.go`):

```go
func (s *TaskService) ReconcileAgentStatus(ctx context.Context, agentID pgtype.UUID) {
    agent, err := s.Queries.RefreshAgentStatusFromTasks(ctx, agentID)
    // ...
}
```

**SQL** (`server/pkg/db/queries/agent.sql`):

```sql
-- RefreshAgentStatusFromTasks
UPDATE agent AS a
SET status = CASE WHEN EXISTS (
    SELECT 1 FROM agent_task_queue q
    WHERE q.agent_id = a.id AND q.status IN ('dispatched', 'running')
) THEN 'working' ELSE 'idle' END,
    updated_at = now()
WHERE a.id = $1
RETURNING *;
```

**Problem:**  
`ReconcileAgentStatus` is called on **every task lifecycle event**:

| Call site | Frequency |
|---|---|
| `ClaimTask` (after successful claim) | Every task dispatch |
| `CompleteTask` | Every task completion |
| `FailTask` | Every task failure |
| `CancelTask` | Every manual cancel |
| `CancelTasksForIssue` (per-row loop) | Once per cancelled task row |
| `CancelTasksByTriggerComment` (per-row loop) | Once per cancelled task row |
| `HandleFailedTasks` (per unique agent) | Every sweep tick with failures |

Each call issues:
1. An `UPDATE agent` with a correlated `EXISTS (SELECT 1 FROM agent_task_queue ...)` subquery
2. A WS event publish (in-memory, cheap)

Under high concurrency — many tasks completing simultaneously across N agents — this produces a burst of concurrent single-row `UPDATE agent WHERE id = $1`. Each write acquires a row-level lock on the `agent` row, so concurrent updates for the same agent serialize. This is a **write hotspot** on `agent` rows for busy agents.

Additionally, `CancelTasksForIssue` and `CancelTasksByTriggerComment` call `ReconcileAgentStatus` once **per cancelled task row** rather than once per unique agent after the batch. If issue A has 3 tasks belonging to agent X, that's 3 consecutive reconcile calls for the same agent row.

**Scale Threshold:**
- **Concurrent tasks per agent:** 3+ (controlled by `max_concurrent_tasks`; default is 2)
- **Agents:** 20+ actively working simultaneously
- **Task throughput:** 50+ task lifecycle events per minute across the workspace (e.g. 20 agents each completing a task every ~25 seconds)
- **Symptom onset:** row-level lock contention on `agent` becomes measurable when a single agent has multiple tasks completing/failing within the same 100ms window. In practice, this is most visible during batch cancellations (e.g. archiving a project cancels all its issues' tasks simultaneously) — a 10-issue cancel with 2 tasks each = 20 redundant reconcile calls in a burst.

**Contrast:** `CancelTasksForAgent` was correctly written to call `ReconcileAgentStatus` once after the loop.

**Recommendation:**
1. In `CancelTasksForIssue` and `CancelTasksByTriggerComment`: collect `affectedAgents` (deduplicated set) then reconcile once per agent after the loop — same pattern as `HandleFailedTasks` and `CancelTasksForAgent`.
2. Consider debouncing reconcile writes: if the same agent has N events in a 100ms window, one reconcile is sufficient. A simple in-memory coalescing map with a goroutine flushing every 50ms would eliminate the N-per-burst writes.
3. The `EXISTS (SELECT 1 FROM agent_task_queue ...)` subquery is backed by `idx_agent_task_queue_agent(agent_id, status)`, so the read side is fast. The bottleneck is the write frequency, not the subquery cost.

---

### 3. Notification fan-out — sequential INSERTs per subscriber, synchronous in event listener

**Code** (`server/cmd/server/notification_listeners.go`, `notifyIssueSubscribers`):

```go
for _, sub := range subs {
    // ...
    item, err := queries.CreateInboxItem(ctx, db.CreateInboxItemParams{...})
    // ...
    bus.Publish(events.Event{Type: protocol.EventInboxNew, ...})
}
```

**Problem:**  
When an issue event triggers subscriber notifications, the code:
1. Fetches the subscriber list (`ListIssueSubscribers`)
2. Iterates over subscribers and calls `CreateInboxItem` **sequentially** for each one — one DB round-trip per subscriber
3. Publishes a WS event per subscriber

This runs synchronously inside the event bus listener goroutine — not in the background. For an issue with N subscribers, this is N sequential `INSERT INTO inbox_item` round-trips, all blocking the listener goroutine before it can process the next event.

High-traffic scenarios:
- A popular issue with 20 subscribers gets a comment → 20 sequential INSERTs
- An agent completes a task on a busy issue → same fan-out
- `@all` mentions → fans out to all workspace members

**Scale Threshold:**
- **Subscribers per issue:** 10+ (each adds ~1–2ms of DB round-trip latency sequentially)
- **Workspace members:** 30+ (for `@all` mention scenarios)
- **Event frequency:** 20+ issue events per minute hitting the same listener goroutine
- **Symptom onset:** with 15 subscribers and ~2ms per INSERT, a single notification fan-out takes ~30ms. If 20 events queue up in the listener goroutine within a second, the backlog reaches 600ms — notifications arrive noticeably late. At 50+ subscribers (large team workspace), a single fan-out takes 100ms+ and the listener becomes a serialization bottleneck.

**Recommendation:**
1. **Batch insert**: replace the sequential loop with a single `INSERT INTO inbox_item (...) VALUES ($1,$2,...), ($3,$4,...), ...` for all recipients at once. sqlc supports `CopyFrom` for bulk inserts; alternatively build a parameterized multi-row INSERT.
2. **Async fan-out**: push the notification job onto a buffered channel or a background goroutine pool so the event listener goroutine returns immediately. The notification delivery can lag by a few hundred milliseconds without user impact.
3. **Preference filter before insert**: the current code loads preferences then filters in Go — this is correct. Ensure `ListNotificationPreferencesByUsers` (the batch preference loader) is itself efficient; it's a bulk lookup by user IDs which is fine.

---

### 4. `ResolveTaskWorkspaceID` — redundant DB read on every task event broadcast

**Code** (`server/internal/service/task.go`):

```go
func (s *TaskService) ResolveTaskWorkspaceID(ctx context.Context, task db.AgentTaskQueue) string {
    if task.IssueID.Valid {
        if issue, err := s.Queries.GetIssue(ctx, task.IssueID); err == nil {
            return util.UUIDToString(issue.WorkspaceID)
        }
    }
    if task.ChatSessionID.Valid {
        if cs, err := s.Queries.GetChatSession(ctx, task.ChatSessionID); err == nil {
            return util.UUIDToString(cs.WorkspaceID)
        }
    }
    // ... autopilot: 2 more DB calls
}
```

**Call sites:**
- `broadcastTaskEvent` — called after every status transition (claim, start, complete, fail, cancel)
- `broadcastTaskDispatch` — called after every claim
- `HandleFailedTasks` — called per failed task

**Problem:**  
For every task event, the service re-queries `GetIssue` (or `GetChatSession`, or 2 calls for autopilot) just to recover the `workspace_id` — information that was already available at task creation time and that the `agent_task_queue` row doesn't carry directly.

A task completion event triggers: `CompleteTask` → `broadcastChatDone` → `broadcastTaskEvent` — that's 2 calls to `ResolveTaskWorkspaceID`, each potentially fetching the same issue row twice.

**Scale Threshold:**
- **Concurrent tasks completing:** 10+ per second across the workspace
- **Chat sessions:** 20+ simultaneous chat tasks (each completion triggers 2 resolve calls)
- **Symptom onset:** each `ResolveTaskWorkspaceID` call is a single `GetIssue` by PK (~0.5ms on warm cache). At 50 task events/second, that's 50–100 extra PK lookups/second — not catastrophic, but wasteful. The real cost is latency on the broadcast path: the WS event delivery to clients is delayed by the DB call, making the UI feel sluggish. Denormalizing `workspace_id` eliminates this entirely for zero ongoing cost.

**Recommendation:**
1. Denormalize `workspace_id` onto `agent_task_queue`. This is the cleanest fix: add the column (migration + sqlc regenerate), populate at insert time, and eliminate `ResolveTaskWorkspaceID` entirely for the common paths.
2. Short-term: add an in-process LRU cache keyed by `task_id → workspace_id`. Task-to-workspace mapping is immutable after creation, so a bounded cache (e.g. 10k entries) with no expiry is safe.

---

## MEDIUM

### 5. `ClaimTask` — 4 sequential DB round-trips per claim attempt

**Code** (`server/internal/service/task.go`, `ClaimTask`):

```go
agent, err := s.Queries.GetAgent(ctx, agentID)           // 1
running, err := s.Queries.CountRunningTasks(ctx, agentID) // 2
task, err := s.Queries.ClaimAgentTask(ctx, agentID)       // 3
s.ReconcileAgentStatus(ctx, agentID)                      // 4
```

**Problem:**  
Each claim attempt requires 4 sequential round-trips. The daemon polls `/tasks/claim` every 30 seconds per runtime, and on wakeup (via WS notification). For a runtime with 10 agents, the claim loop in `ClaimTaskForRuntime` iterates over candidate agents, calling `ClaimTask` for each — potentially 4 × N round-trips per poll cycle.

**Scale Threshold:**
- **Runtimes (daemons):** 5+ polling simultaneously
- **Agents per runtime:** 10+ (each candidate agent = 4 round-trips in the worst case)
- **Task queue depth:** 20+ queued tasks across multiple agents on the same runtime
- **Symptom onset:** with local PG (~0.5ms per round-trip), 4 × 10 agents = 20ms per poll cycle — negligible. With remote PG (5ms per round-trip), that's 200ms per poll cycle. The `EmptyClaimCache` makes this a non-issue in steady state (no queued tasks = instant return). Only matters during burst enqueue scenarios where many tasks land simultaneously and multiple runtimes race to claim.

**Mitigations already in place:**
- `FOR UPDATE SKIP LOCKED` in `ClaimAgentTask`: losers don't block, they get an empty result immediately. Lock wait is not the problem.
- `EmptyClaimCache` (Redis): if the runtime had no queued tasks on the last check, subsequent polls short-circuit without touching Postgres.
- `ListQueuedClaimCandidatesByRuntime` backed by `idx_agent_task_queue_claim_candidates` (partial index on `status = 'queued'`): the candidate list fetch is cheap.
- `triedAgents` map: prevents retry on the same agent within a single poll cycle.
- `NOT EXISTS` correlated subquery in `ClaimAgentTask` backed by `idx_agent_task_queue_agent(agent_id, status)`: index-only scan, fast.

**Remaining concerns:**
- `GetAgent` and `CountRunningTasks` could be collapsed into a single query that returns both the agent row and the running count, saving one round-trip.
- `ReconcileAgentStatus` after every claim (see item #2 above) adds latency to the claim hot path. The `maybeLogClaimSlow` telemetry (>300ms threshold) already tracks this.

**Recommendation:**
- Merge `GetAgent` + `CountRunningTasks` into one query that returns the agent with a count lateral join. Net: 3 round-trips per claim instead of 4.
- The `EmptyClaimCache` already handles the zero-task steady state efficiently — the current design is sound, the above is a future optimization.

---

### 6. `GetWorkspaceAgentRunCounts` + `GetWorkspaceAgentActivity30d` — time-range scans without a covering index

**Queries** (`server/pkg/db/queries/agent.sql`):

```sql
-- GetWorkspaceAgentRunCounts
SELECT atq.agent_id, COUNT(*)::int AS run_count
FROM agent_task_queue atq
JOIN agent a ON a.id = atq.agent_id
WHERE a.workspace_id = $1
  AND atq.created_at > now() - INTERVAL '30 days'
GROUP BY atq.agent_id;

-- GetWorkspaceAgentActivity30d
SELECT atq.agent_id, DATE_TRUNC('day', atq.completed_at)::timestamptz AS bucket, ...
FROM agent_task_queue atq
JOIN agent a ON a.id = atq.agent_id
WHERE a.workspace_id = $1
  AND atq.completed_at IS NOT NULL
  AND atq.completed_at > now() - INTERVAL '30 days'
GROUP BY atq.agent_id, bucket;
```

**Problem:**  
Both queries JOIN through `agent` to scope by workspace, then filter by a 30-day time window on `created_at` / `completed_at`. Existing indexes:
- `idx_agent_task_queue_agent(agent_id, status)` — does not help time-range scans
- `idx_agent_task_queue_claim_candidates` — partial on `status = 'queued'`, irrelevant here
- No index on `(agent_id, created_at)` or `(agent_id, completed_at)`

For a workspace with many agents each running hundreds of tasks per day, the 30-day window can be a large range scan followed by an aggregation. Both queries hit every qualifying row to compute counts.

**Scale Threshold:**
- **Tasks per agent per day:** 50+ (e.g. 20 agents × 50 tasks/day × 30 days = 30k rows scanned)
- **Total 30-day task rows:** 30k+ in the workspace
- **Symptom onset:** these queries run once per Agents page load (not on hot path). At 30k rows the scan takes ~50ms — noticeable but acceptable. At 100k+ rows (high-throughput workspace running agents on many small tasks), expect 200ms+ page load contribution. The `task_usage_daily` rollup table already exists as the correct long-term fix.

**Recommendation:**
1. Add `(agent_id, created_at DESC)` and `(agent_id, completed_at DESC)` indexes on `agent_task_queue`. This lets the planner seek directly into the 30-day window per agent.
2. These queries back the Agents list page and are called once per page load — not on the hot path. The existing `task_usage` daily rollup table (`073_task_usage_daily_rollup`) is the right long-term answer: pre-aggregate completions daily and serve the Agents list from the rollup. `GetWorkspaceAgentRunCounts` could then become a rollup scan bounded by 30 rows (30 days × 1 agent), not 30k.

---

### 7. `inbox_item` — missing index on `issue_id` column

**Affected queries:**

```sql
-- ArchiveInboxByIssue
UPDATE inbox_item SET archived = true
WHERE workspace_id = $1 AND recipient_type = $2 AND recipient_id = $3 AND issue_id = $4 AND archived = false;

-- ArchiveInboxByIssueAndType
UPDATE inbox_item SET archived = true
WHERE workspace_id = $1 AND issue_id = $2 AND type = $3 AND archived = false
RETURNING recipient_type, recipient_id;

-- ArchiveCompletedInbox
UPDATE inbox_item i SET archived = true
WHERE i.workspace_id = $1 AND i.recipient_type = 'member' AND i.recipient_id = $2 AND i.archived = false
  AND i.issue_id IN (SELECT id FROM issue WHERE status IN ('done', 'cancelled'));
```

**Problem:**  
The only index on `inbox_item` is `idx_inbox_recipient(recipient_type, recipient_id, read)`. Queries that filter by `issue_id` — such as bulk-archiving when an issue is closed — cannot use this index and must scan all inbox items for the workspace. As a user's inbox grows (many issues, many events), these UPDATE scans become expensive.

`ArchiveInboxByIssueAndType` is called on every agent task completion/failure to auto-archive prior agent-activity notifications for the same issue. This runs on the task completion hot path.

**Scale Threshold:**
- **Inbox items per workspace:** 10k+ unarchived rows
- **Issues with repeated agent runs:** issues that have been retried or reassigned multiple times accumulate many inbox items per issue
- **Symptom onset:** without an index, `ArchiveInboxByIssueAndType` does a sequential scan filtered by `(workspace_id, issue_id, type, archived=false)`. At 10k unarchived inbox items, this scan takes ~5–10ms. At 50k+ (workspace with 30 members, each accumulating months of notifications), expect 30–50ms per task completion — directly on the hot path. The fix is a single-line migration.

**Existing index coverage:**
- `ListInboxItems` + `CountUnreadInbox` filter by `(recipient_type, recipient_id)` — covered by `idx_inbox_recipient`.
- `MarkAllInboxRead`, `ArchiveAllInbox`, `ArchiveAllReadInbox` filter by `(workspace_id, recipient_type, recipient_id)` — partially covered (recipient columns match the index prefix).

**Recommendation:**
Add a partial index:
```sql
CREATE INDEX idx_inbox_issue_id ON inbox_item (issue_id, workspace_id)
WHERE archived = false;
```
This makes `ArchiveInboxByIssue`, `ArchiveInboxByIssueAndType`, and `ArchiveCompletedInbox` efficient. The `WHERE archived = false` partial keeps the index small since archived rows are the majority over time.

---

## LOW

### 8. `HandleFailedTasks` — N+1 `GetIssue` per failed task batch

**Code** (`server/internal/service/task.go`):

```go
for _, t := range tasks {
    // ...
    if t.IssueID.Valid {
        if issue, err := s.Queries.GetIssue(ctx, t.IssueID); err == nil {
            workspaceID = util.UUIDToString(issue.WorkspaceID)
            // check status, maybe UpdateIssueStatus
        }
    }
}
```

**Problem:**  
When the sweeper or daemon delivers a batch of failed tasks, `HandleFailedTasks` calls `GetIssue` once per task. A batch of 50 timed-out tasks belonging to 10 distinct issues = up to 50 `GetIssue` calls (some repeated for the same issue).

The `processedIssues` map deduplicates the `HasActiveTaskForIssue` + `UpdateIssueStatus` path per issue, but the `GetIssue` call itself is not deduplicated.

**Impact:** The sweeper runs every 30 seconds. Under normal operation the batch size is small. This only matters if the sweeper encounters a large backlog of expired tasks (e.g. after a daemon outage).

**Scale Threshold:**
- **Failed task batch size:** 20+ tasks in a single sweep (normal operation: 0–3)
- **Trigger scenario:** daemon offline for 1+ hours while tasks queue up and hit the 5-minute dispatch timeout → sweeper finds 50+ expired tasks at once
- **Symptom onset:** at batch size 50 with 10 distinct issues, that's 50 `GetIssue` calls (~25ms total with warm PG cache). Barely noticeable. Only worth fixing as a code hygiene improvement when the function is next modified.

**Recommendation:**
Build a `map[string]db.Issue` from the unique `IssueID` values in the batch before the loop, fetching each issue once. At batch sizes of 10–50 this has negligible impact today; worth fixing when `HandleFailedTasks` is next touched.

---

### 9. `ListIssues` — OFFSET pagination degrades at high page numbers

**Query** (`server/pkg/db/queries/issue.sql`):

```sql
SELECT ... FROM issue
WHERE workspace_id = $1 AND ...
ORDER BY position ASC, created_at DESC
LIMIT $2 OFFSET $3;
```

**Problem:**  
`OFFSET N` forces Postgres to read and discard N rows before returning results. At page 100 with page size 50, that's 5000 rows discarded. For workspaces with thousands of issues this degrades linearly with page number.

**Impact:** Low today given typical workspace sizes (hundreds of issues). Becomes relevant at multi-thousand-issue workspaces.

**Scale Threshold:**
- **Issues per workspace:** 5,000+ (at page size 50, page 100 = OFFSET 5000)
- **Symptom onset:** OFFSET 1000 adds ~5ms on a well-indexed table; OFFSET 5000 adds ~20ms; OFFSET 10000 adds ~50ms. In practice, the frontend uses board view (loads all) or list view (users rarely paginate past page 5 = OFFSET 250). This becomes a real problem only for API consumers doing bulk exports or automated scans across all issues. The CLI's `--offset` flag makes this theoretically reachable but unlikely in normal usage.

**Recommendation:**  
Migrate to keyset (cursor) pagination: `WHERE (position, created_at, id) > ($last_position, $last_created_at, $last_id)`. The comment timeline endpoint already uses this pattern (migration `068_timeline_keyset_index`); the same approach applies to issue listing.

---

## Appendix: Main Branch Validation Notes (2026-05-27)

Re-evaluated all findings against `main` (commit `5f1f08e4`, migrations up to 107). Key observations:

### No Findings Fixed

All 9 issues remain structurally identical on main. The `agent_task_queue` table still lacks `workspace_id`; `ReconcileAgentStatus` is still called per-row in `CancelTasksForIssue`/`CancelTasksByTriggerComment`; notification fan-out is still sequential; `ResolveTaskWorkspaceID` still re-queries; `ClaimTask` still does 4 round-trips; no covering indexes for 30-day queries; no `issue_id` index on `inbox_item`; `HandleFailedTasks` still has N+1; `ListIssues` still uses OFFSET.

### New Indexes (migrations 081–107, not covering identified gaps)

| Migration | Index | Purpose |
|---|---|---|
| 084 | `idx_squad_workspace`, `idx_squad_member_squad`, `idx_squad_member_entity` | Squad feature |
| 089 | `idx_activity_log_squad_no_action_task` | Squad leader evaluation |
| 101 | Task usage hourly indexes (workspace/time, runtime/time, agent/time, project/time) | Usage dashboard |
| 105 | `idx_issue_metadata_gin` | Issue metadata JSONB filtering |
| 106 | `idx_member_user_workspace` | Member lookup by user+workspace |

None of these address the gaps identified in this document (`workspace_id` on task queue, `(agent_id, created_at/completed_at)` indexes, `inbox_item.issue_id` index).

### New Complexity on Main (potential additional concerns)

**#9 addendum — `ListIssues` query is heavier on main:**  
The `involves_user_id` filter now includes 3 UNION subqueries against `squad`, `squad_member`, and `agent` tables to resolve indirect assignment through squads. This makes the query more expensive when the filter is active, though it only fires when the `involves_user_id` parameter is non-NULL (My Issues page "All" scope).

**Squad leader evaluation queries:**  
New queries like `GetLatestTaskIsLeaderForIssueAndAgent` scan `agent_task_queue` by `(issue_id, agent_id)` with `ORDER BY created_at DESC LIMIT 1`. The existing `idx_one_pending_task_per_issue_agent` partial index only covers `status IN ('queued', 'dispatched')` and won't help this query. A general `(issue_id, agent_id, created_at DESC)` index would serve both this and the existing `HasPendingTaskForIssueAndAgent`.

**`ClaimTask` instrumentation:**  
Main added `maybeLogClaimSlow` telemetry (logs when total claim latency exceeds 300ms), confirming the team is aware of the latency concern in finding #5 but has not yet restructured the sequential round-trips.

**Task usage hourly pipeline (migrations 101–103):**  
Replaces the legacy daily rollup with an hourly pipeline. However, `GetWorkspaceAgentRunCounts` and `GetWorkspaceAgentActivity30d` (finding #6) still query `agent_task_queue` directly rather than reading from the new rollup tables — the optimization opportunity remains unrealized.

---

## Index Reference

Current indexes on the hot tables (`agent_task_queue`, `inbox_item`):

| Index | Columns | Partial condition | Backs |
|---|---|---|---|
| `idx_agent_task_queue_agent` | `(agent_id, status)` | — | `CountRunningTasks`, `NOT EXISTS` in `ClaimAgentTask` |
| `idx_agent_task_queue_claim_candidates` | `(runtime_id, priority DESC, created_at ASC)` | `status = 'queued'` | `ListQueuedClaimCandidatesByRuntime` |
| `idx_agent_task_queue_queued_created_at` | `(created_at)` | `status = 'queued'` | `ExpireStaleQueuedTasks` sweeper |
| `idx_agent_task_queue_issue_id` | `(issue_id)` | — | Cross-status issue aggregations |
| `idx_one_pending_task_per_issue_agent` | `(issue_id, agent_id)` | `status IN ('queued', 'dispatched')` | Uniqueness constraint |
| `idx_inbox_recipient` | `(recipient_type, recipient_id, read)` | — | `ListInboxItems`, `CountUnreadInbox` |

**Gaps identified:**
- No `workspace_id` on `agent_task_queue` → workspace-scoped queries require JOIN through `agent`
- No `(agent_id, created_at)` or `(agent_id, completed_at)` → 30-day activity scans are unguided
- No `issue_id` index on `inbox_item` → bulk archive by issue requires scan

---

## Summary Table

| # | Finding | Severity | Effort | Impact |
|---|---|---|---|---|
| 1 | `ListWorkspaceAgentTaskSnapshot` workspace scan + unbounded DISTINCT ON | **CRITICAL** | High | All agent real-time state views |
| 2 | `ReconcileAgentStatus` write hotspot + per-row call in cancel loops | **HIGH** | Low | Every task lifecycle event |
| 3 | Notification fan-out: sequential INSERTs per subscriber, synchronous | **HIGH** | Medium | Every comment/assignment event |
| 4 | `ResolveTaskWorkspaceID`: redundant `GetIssue` on every broadcast | **HIGH** | Low | Every task event broadcast |
| 5 | `ClaimTask`: 4 sequential round-trips per attempt | **MEDIUM** | Medium | Daemon poll cycle |
| 6 | 30-day activity queries: no `(agent_id, created_at/completed_at)` index | **MEDIUM** | Low | Agents list page load |
| 7 | `inbox_item`: no `issue_id` index, archive-by-issue scans | **MEDIUM** | Low | Task completion + issue close |
| 8 | `HandleFailedTasks`: repeated `GetIssue` per task in batch | **LOW** | Low | Sweeper batch processing |
| 9 | `ListIssues` OFFSET pagination | **LOW** | High | High-page-number queries |
