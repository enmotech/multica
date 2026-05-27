# Task Dispatch Mechanism

> Reviewed: 2026-05-27  
> Branch: main (validated against moclaw fork)  
> Scope: Issue → Agent task lifecycle, from user action to agent execution

This document describes how multica dispatches tasks to agents — from the moment a user changes an issue status to the agent starting work.

---

## Overview

```
User action (UI/API)
  → Server: enqueue task (INSERT into agent_task_queue, status = 'queued')
  → Server: broadcast WS event (task:queued) + wakeup daemon
  → Daemon: receives wakeup, calls POST /tasks/claim
  → Server: ClaimTask (status → 'dispatched'), returns task + agent data
  → Daemon: prepares execution environment (worktree, skill files, runtime config)
  → Daemon: spawns agent runtime process (e.g. claude, codex)
  → Agent: calls `multica issue status <id> in_progress`
  → User sees issue move to "In Progress" in the UI
```

---

## Trigger Conditions

Tasks are enqueued via `TaskService.EnqueueTaskForIssue` under these conditions:

### 1. Issue created with agent assignee

When a new issue is created and assigned to an agent, as long as the status is not `backlog`:

```go
// handler/issue.go — CreateIssue
if issue.AssigneeType.Valid && issue.AssigneeID.Valid {
    if h.shouldEnqueueAgentTask(r.Context(), issue) {
        h.TaskService.EnqueueTaskForIssue(r.Context(), issue)
    }
}
```

### 2. Assignee changed to an agent

When an existing issue's assignee is changed to an agent (and status is not `backlog`):

```go
// handler/issue.go — UpdateIssue
if assigneeChanged {
    h.TaskService.CancelTasksForIssue(r.Context(), issue.ID)
    if h.shouldEnqueueAgentTask(r.Context(), issue) {
        h.TaskService.EnqueueTaskForIssue(r.Context(), issue)
    }
}
```

### 3. Issue moved out of backlog

When a **member** (not an agent) moves an issue from `backlog` to any active status (todo, in_progress, etc.), and the issue already has an agent assignee:

```go
// handler/issue.go — UpdateIssue
if statusChanged && !assigneeChanged && actorType == "member" &&
    prevIssue.Status == "backlog" && issue.Status != "done" && issue.Status != "cancelled" {
    if h.isAgentAssigneeReady(r.Context(), issue) {
        h.TaskService.EnqueueTaskForIssue(r.Context(), issue)
    }
}
```

**This is the "drag to TODO" scenario.** Backlog acts as a parking lot — issues can be pre-assigned to agents without triggering execution.

### 4. Comment on an agent-assigned issue

When a member posts a comment on an issue assigned to an agent:

```go
// handler/comment.go
if _, err := h.TaskService.EnqueueTaskForIssue(r.Context(), issue, comment.ID); err != nil {
    slog.Warn("enqueue agent task on comment failed", ...)
}
```

### 5. @mention of an agent in a comment

When an agent is @mentioned in a comment, a task is enqueued for that specific agent (even if it's not the assignee):

```go
h.TaskService.EnqueueTaskForMention(r.Context(), issue, agentID, triggerCommentID)
```

---

## Guard Conditions

### `shouldEnqueueAgentTask`

```go
func (h *Handler) shouldEnqueueAgentTask(ctx context.Context, issue db.Issue) bool {
    if issue.Status == "backlog" {
        return false
    }
    return h.isAgentAssigneeReady(ctx, issue)
}
```

### `isAgentAssigneeReady`

```go
func (h *Handler) isAgentAssigneeReady(ctx context.Context, issue db.Issue) bool {
    if !issue.AssigneeType.Valid || issue.AssigneeType.String != "agent" || !issue.AssigneeID.Valid {
        return false
    }
    agent, err := h.Queries.GetAgent(ctx, issue.AssigneeID)
    if err != nil || !agent.RuntimeID.Valid || agent.ArchivedAt.Valid {
        return false
    }
    return true
}
```

Checks:
- Issue is assigned to an agent (not a member or squad)
- Agent exists and has a runtime bound
- Agent is not archived

---

## Enqueue Phase

`EnqueueTaskForIssue` performs:

1. **Validate** — issue has an assignee, agent exists, agent has a runtime, agent is not archived
2. **INSERT** — creates a row in `agent_task_queue` with `status = 'queued'`
3. **Broadcast** — publishes `task:queued` WS event (frontend updates immediately)
4. **Wakeup** — notifies the daemon via `notifyTaskAvailable`

```go
func (s *TaskService) enqueueIssueTask(ctx context.Context, issue db.Issue, ...) (db.AgentTaskQueue, error) {
    // ... validation ...
    task, err := s.Queries.CreateAgentTask(ctx, db.CreateAgentTaskParams{
        AgentID:   issue.AssigneeID,
        RuntimeID: agent.RuntimeID,
        IssueID:   issue.ID,
        Priority:  priorityToInt(issue.Priority),
        // ...
    })
    s.broadcastTaskEvent(ctx, protocol.EventTaskQueued, task)
    s.NotifyTaskEnqueued(ctx, task)
    return task, nil
}
```

### Wakeup mechanism

```go
func (s *TaskService) notifyTaskAvailable(task db.AgentTaskQueue) {
    runtimeKey := util.UUIDToString(task.RuntimeID)
    // Invalidate the "no tasks" cache so daemon doesn't skip this poll
    s.EmptyClaim.Bump(context.Background(), runtimeKey)
    // Kick the daemon's WS connection
    s.Wakeup.NotifyTaskAvailable(runtimeKey, util.UUIDToString(task.ID))
}
```

---

## Daemon Polling & Claim

### Polling model: wakeup + fallback timer

The daemon runs one `runRuntimePoller` goroutine per registered runtime. Each poller:

1. Waits for either a **wakeup signal** (via WS) or a **30-second timer** (fallback)
2. Acquires an execution slot (semaphore) **before** claiming — prevents dispatched tasks from piling up
3. Calls `POST /api/daemon/runtimes/{runtimeId}/tasks/claim`
4. If a task is returned, spawns a goroutine to handle it; loops immediately for more
5. If no task, goes back to sleep

```go
// daemon.go — runRuntimePoller (simplified)
for {
    slot := <-sem  // acquire execution slot
    task, err := d.client.ClaimTask(pollerCtx, rid)
    if task == nil {
        sem <- slot  // release slot
        sleepWithContextOrWakeup(pollerCtx, 30*time.Second, wakeup)
        continue
    }
    go d.handleTask(parentCtx, *task, slot)
    // loop immediately — more tasks may be queued
}
```

```go
// Default poll interval (fallback when wakeup is missed)
DefaultPollInterval = 30 * time.Second
```

### Wakeup fan-out

When `taskWakeups` channel receives a signal, the `pollLoop` fans it out to all runtime pollers:

```go
case <-taskWakeups:
    for _, h := range pollers {
        select {
        case h.wakeup <- struct{}{}:
        default:  // non-blocking, coalesces bursts
        }
    }
```

### `sleepWithContextOrWakeup`

The sleep is interruptible by three events:

```go
select {
case <-ctx.Done():   return ctx.Err()  // shutdown
case <-wakeups:      return nil         // new task available
case <-timer.C:      return nil         // 30s fallback expired
}
```

---

## Claim Phase (Server Side)

### `ClaimTaskForRuntime` — two-stage claim

1. **List candidates**: `ListQueuedClaimCandidatesByRuntime` — cheap SELECT backed by partial index `idx_agent_task_queue_claim_candidates`
2. **Try each agent**: iterates candidate agents (deduplicated), calls `ClaimTask` per agent

### `ClaimTask` — 4 sequential steps

```go
func (s *TaskService) ClaimTask(ctx context.Context, agentID pgtype.UUID) (*db.AgentTaskQueue, error) {
    agent := s.Queries.GetAgent(ctx, agentID)                    // 1. load agent
    running := s.Queries.CountRunningTasks(ctx, agentID)         // 2. check capacity
    if running >= agent.MaxConcurrentTasks { return nil, nil }
    task := s.Queries.ClaimAgentTask(ctx, agentID)               // 3. atomic claim
    s.ReconcileAgentStatus(ctx, agentID)                         // 4. update agent status
    s.broadcastTaskDispatch(ctx, task)                           // broadcast task:dispatch
    return &task, nil
}
```

### `ClaimAgentTask` SQL — atomic with `FOR UPDATE SKIP LOCKED`

```sql
UPDATE agent_task_queue
SET status = 'dispatched', dispatched_at = now()
WHERE id = (
    SELECT atq.id FROM agent_task_queue atq
    WHERE atq.agent_id = $1 AND atq.status = 'queued'
      AND NOT EXISTS (
          SELECT 1 FROM agent_task_queue active
          WHERE active.agent_id = atq.agent_id
            AND active.status IN ('dispatched', 'running')
            AND (
              (atq.issue_id IS NOT NULL AND active.issue_id = atq.issue_id)
              OR (atq.chat_session_id IS NOT NULL AND active.chat_session_id = atq.chat_session_id)
              OR (/* quick-create serialization */)
            )
      )
    ORDER BY atq.priority DESC, atq.created_at ASC
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;
```

Key properties:
- **Per-(issue, agent) serialization**: won't dispatch a second task for the same issue+agent if one is already running
- **Priority ordering**: higher priority tasks are claimed first
- **SKIP LOCKED**: concurrent claims don't block each other — losers get empty result immediately
- **Different agents can work the same issue in parallel** (e.g. assignee + @mentioned agent)

### EmptyClaimCache (Redis)

Optimization for the idle case:
- After a poll returns no tasks, the runtime is marked "empty" in Redis
- Subsequent polls short-circuit without touching Postgres
- Cache is invalidated on every `EnqueueTaskForIssue` via version bumping (`Bump`)
- Prevents unnecessary DB load when most runtimes are idle

---

## Execution Phase (Daemon Side)

### Task response payload

The claim response includes everything the daemon needs:

```go
type TaskAgentData struct {
    ID           string                   `json:"id"`
    Name         string                   `json:"name"`
    Instructions string                   `json:"instructions"`
    Skills       []service.AgentSkillData `json:"skills,omitempty"`
    CustomEnv    map[string]string        `json:"custom_env,omitempty"`
    CustomArgs   []string                 `json:"custom_args,omitempty"`
    McpConfig    json.RawMessage          `json:"mcp_config,omitempty"`
    Model        string                   `json:"model,omitempty"`
}
```

Skills are loaded fresh from the DB at claim time (`LoadAgentSkills`), including all supporting files.

### Execution environment setup

The daemon prepares the working directory:

1. **Git worktree** — creates/reuses a worktree for the task
2. **Skill files** — writes each skill as `{provider-path}/skills/{name}/SKILL.md` + supporting files
3. **Runtime config** — generates and writes `CLAUDE.md` or `AGENTS.md` with:
   - Agent identity and instructions
   - Available CLI commands reference
   - Workflow steps (get issue → set in_progress → do work → comment → set in_review)
   - Repository list
   - Project context
   - Mention rules
4. **Issue context** — writes `.agent_context/issue_context.md`

Skill file paths are provider-native:
| Provider | Skill path |
|---|---|
| Claude | `{workDir}/.claude/skills/{name}/SKILL.md` |
| Codex | `{codexHome}/skills/{name}/SKILL.md` |
| Copilot | `{workDir}/.github/skills/{name}/SKILL.md` |
| Cursor | `{workDir}/.cursor/skills/{name}/SKILL.md` |
| Kiro | `{workDir}/.kiro/skills/{name}/SKILL.md` |

### Agent process spawn

The daemon spawns the agent runtime process (e.g. `claude`, `codex`) with:
- Working directory set to the prepared worktree
- Environment variables including `MULTICA_TASK_SLOT`, custom env from agent config
- For Codex: `CODEX_HOME` pointing to the per-task codex-home directory

---

## Agent Workflow (Prompt-Driven)

The agent's behavior is governed by the generated `CLAUDE.md`/`AGENTS.md`. For assignment-triggered tasks:

```markdown
1. Run `multica issue get <id> --output json` to understand your task
2. Run `multica issue comment list <id> --output json` to read comment history
3. Run `multica issue status <id> in_progress`
4. Follow your Skills and Agent Identity to complete the task
5. Post your final results as a comment (mandatory)
6. When done, run `multica issue status <id> in_review`
7. If blocked, run `multica issue status <id> blocked`
```

The agent manages issue status transitions via CLI — the server does not automatically change issue status on task state changes.

---

## Task Lifecycle States

```
queued → dispatched → running → completed
                              → failed (may auto-retry → new queued task)
       → cancelled (at any active state)
```

| Transition | Triggered by |
|---|---|
| → queued | `EnqueueTaskForIssue`, `EnqueueTaskForMention`, `EnqueueChatTask`, `MaybeRetryFailedTask` |
| queued → dispatched | `ClaimAgentTask` (daemon claim) |
| dispatched → running | `StartTask` (daemon reports agent started) |
| running → completed | `CompleteTask` (daemon reports success) |
| running → failed | `FailTask` (daemon reports failure, or sweeper timeout) |
| any active → cancelled | `CancelTask`, `CancelTasksForIssue`, `CancelTasksForAgent` |

### Auto-retry

Failed tasks may be automatically retried (`MaybeRetryFailedTask`):
- Creates a child task with `attempt + 1`
- Respects `max_attempts` (default: 2)
- Resume-unsafe failures (iteration_limit, api_invalid_request, codex_semantic_inactivity) get `force_fresh_session = true`

### Sweeper

A background sweeper (`FailStaleTasks`) runs every 30 seconds:
- Fails tasks stuck in `dispatched` beyond `dispatch_timeout_secs` (default: 300s)
- Fails tasks stuck in `running` beyond `running_timeout_secs` (default: agent timeout)
- Expires tasks stuck in `queued` beyond TTL (`ExpireStaleQueuedTasks`)

---

## Latency Breakdown: TODO → In Progress

When a user drags an issue from Backlog/TODO to an active status, the visible delay before "In Progress" appears is:

| Step | Typical latency | Notes |
|---|---|---|
| Server enqueue + WS wakeup | < 50ms | Instant |
| Daemon receives wakeup | 50–200ms | Network + WS delivery |
| Daemon claim HTTP round-trip | 50–200ms | 4 DB queries |
| Execution env setup | 1–3s | Git worktree + file writes |
| Agent process startup | 1–3s | Runtime initialization |
| Agent reads issue + comments | 1–2s | 2 CLI calls |
| Agent sets status to in_progress | 0.5–1s | 1 CLI call |
| **Total** | **~3–8s** | Dominated by env setup + agent startup |

The delay is not a polling lag (wakeup is near-instant) but rather the cold-start cost of preparing the execution environment and spawning the agent process.

---

## Concurrency Controls

| Control | Mechanism |
|---|---|
| Per-agent concurrency | `max_concurrent_tasks` on agent row; checked in `ClaimTask` before claiming |
| Per-(issue, agent) serialization | `NOT EXISTS` subquery in `ClaimAgentTask`; prevents duplicate work |
| Per-runtime slot limit | Semaphore in daemon `pollLoop`; slot acquired before claim |
| Unique pending constraint | DB index `idx_one_pending_task_per_issue_agent` (partial, queued+dispatched) |
| Comment dedup | `HasPendingTaskForIssueAndAgent` check before enqueue on comment |

---

## Key Configuration

| Parameter | Default | Env var | Description |
|---|---|---|---|
| Poll interval | 30s | `MULTICA_DAEMON_POLL_INTERVAL` | Fallback polling interval (wakeup bypasses this) |
| Max concurrent tasks (daemon) | configurable | `MULTICA_MAX_CONCURRENT_TASKS` | Total execution slots per daemon |
| Max concurrent tasks (agent) | 2 | Set per agent in UI | Per-agent parallelism limit |
| Dispatch timeout | 300s | — | Server fails dispatched tasks not started within this window |
| Agent timeout | 2h | `MULTICA_AGENT_TIMEOUT` | Max runtime for a single task |
