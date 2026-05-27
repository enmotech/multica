# Autopilot 调度机制

Autopilot 是 Multica 的自动化任务执行系统，允许用户配置定时触发的 Agent 任务。整个调度机制分为四层：**数据模型**、**调度器轮询**、**触发执行**、**结果同步**。

## 数据模型

调度涉及三张核心表：

```
autopilot                        autopilot_trigger                 autopilot_run
┌─────────────────────┐          ┌─────────────────────────┐      ┌──────────────────────┐
│ id                  │◄────┐    │ id                      │      │ id                   │
│ workspace_id        │     │    │ autopilot_id ──────────►│      │ autopilot_id         │
│ execution_mode      │     │    │ kind (schedule/webhook) │      │ trigger_id           │
│ assignee_id → agent │     │    │ cron_expression         │      │ source               │
│ title               │     │    │ timezone                │      │ status               │
│ description         │     │    │ next_run_at             │      │ issue_id             │
│ issue_title_template│     │    │ last_fired_at           │      │ task_id              │
│ status (active/...) │     │    │ enabled                 │      │ triggered_at         │
│ last_run_at         │     └────│ label                   │      │ completed_at         │
└─────────────────────┘          └─────────────────────────┘      │ failure_reason       │
                                                                   └──────────────────────┘
```

### 关键字段说明

| 表 | 字段 | 说明 |
|---|---|---|
| `autopilot` | `execution_mode` | `create_issue`（创建 issue 后入队）或 `run_only`（直接入 task queue） |
| `autopilot` | `assignee_id` | 执行任务的 Agent |
| `autopilot_trigger` | `cron_expression` | 标准 5 字段 cron 表达式（分 时 日 月 周） |
| `autopilot_trigger` | `timezone` | 用户配置的时区（如 `Asia/Tokyo`），默认 `UTC` |
| `autopilot_trigger` | `next_run_at` | 下次触发时间（`timestamptz`），调度器据此判断是否到期 |
| `autopilot_run` | `status` | `issue_created` / `running` / `completed` / `failed` / `skipped` |

## 调度器轮询（Scheduler）

调度器位于 `server/cmd/server/autopilot_scheduler.go`，是一个随服务器启动的后台 goroutine。

### 运行机制

```
服务器启动
    │
    ├─ recoverLostTriggers()   ← 崩溃恢复
    │
    └─ 每 30 秒 tick 一次
         └─ tickScheduledAutopilots()
```

### Tick 流程

每次 tick 执行以下三步：

#### Step 1: Claim（原子抢占）

执行 `ClaimDueScheduleTriggers` SQL：

```sql
UPDATE autopilot_trigger t
SET next_run_at = NULL
FROM autopilot a
WHERE t.autopilot_id = a.id
  AND t.kind = 'schedule'
  AND t.enabled = true
  AND t.next_run_at IS NOT NULL
  AND t.next_run_at <= now()
  AND a.status = 'active'
RETURNING t.*, a.workspace_id;
```

将 `next_run_at` 置为 NULL 相当于"锁定"该 trigger，利用 PostgreSQL 行级锁保证**多实例部署时不会重复触发**。

#### Step 2: Dispatch（逐个派发）

对每个 claimed trigger，加载对应的 autopilot，调用 `DispatchAutopilot()`。

#### Step 3: Advance（推进下次时间）

使用 `robfig/cron/v3` 库，结合 trigger 的 `cron_expression` 和 `timezone`，计算下一次触发时间并写回 `next_run_at`：

```go
func ComputeNextRun(cronExpr, timezone string) (time.Time, error) {
    sched, _ := cronParser.Parse(cronExpr)
    loc, _ := time.LoadLocation(timezone)
    return sched.Next(time.Now().In(loc)), nil
}
```

### 崩溃恢复

服务器启动时执行 `recoverLostTriggers()`：扫描所有 `next_run_at = NULL`（被 claim 但未 advance）的 schedule trigger，重新计算 `next_run_at` 并写回。这处理了调度器在 claim 之后、advance 之前崩溃的场景。

## 触发执行（Dispatch）

`server/internal/service/autopilot.go` → `DispatchAutopilot()`

### 执行流程

```
DispatchAutopilot(ctx, autopilot, triggerID, source, payload)
    │
    ├─ 准入检查 (shouldSkipDispatch)
    │    ├─ autopilot 无 assignee → skip
    │    ├─ agent 已归档 → skip
    │    ├─ agent 未绑定 runtime → skip
    │    └─ runtime 不在线 → skip
    │    （skip 时记录 skipped run，不入队）
    │
    ├─ 创建 autopilot_run 记录
    │
    ├─ 根据 execution_mode 分支：
    │    │
    │    ├─ "create_issue" → dispatchCreateIssue()
    │    │    ├─ 生成 issue title（支持 {{date}} 模板变量）
    │    │    ├─ 生成 issue description（附加触发时间戳）
    │    │    ├─ 在事务中创建 issue（带 origin_type=autopilot）
    │    │    ├─ 发布 issue:created 事件（触发通知等下游监听）
    │    │    └─ TaskSvc.EnqueueTaskForIssue() → 入 agent_task_queue
    │    │
    │    └─ "run_only" → dispatchRunOnly()
    │         ├─ 直接创建 agent_task_queue 记录（不创建 issue）
    │         └─ NotifyTaskEnqueued() → 唤醒 daemon 拉取任务
    │
    ├─ 更新 autopilot.last_run_at
    └─ 发布 autopilot_run:start WebSocket 事件
```

### 准入检查（Admission Gate）

MUL-1899 引入的保护机制。在 dispatch 前检查 agent 的 runtime 是否在线：

- 如果 agent 的 runtime 离线（如笔记本合盖），直接记录 `skipped` run 并返回
- 避免往 task queue 中堆积大量注定失败的任务
- DB 查询失败时**不跳过**（宁可尝试执行，也不因瞬时 DB 问题丢失调度）

### 两种执行模式

| 模式 | 行为 | 适用场景 |
|------|------|---------|
| `create_issue` | 创建 issue → 分配给 agent → 入 task queue | 需要完整工作流追踪的任务 |
| `run_only` | 直接入 task queue，不创建 issue | 轻量级、无需追踪的自动化任务 |

## 结果同步

Autopilot 不轮询结果，而是通过**事件驱动**反向同步 run 状态。

### create_issue 模式

`SyncRunFromIssue()` — 监听 issue 状态变更：

| Issue 状态 | Run 状态 |
|-----------|---------|
| `done` / `in_review` | `completed` |
| `cancelled` / `blocked` | `failed` |

### run_only 模式

`SyncRunFromTask()` — 监听 agent_task_queue 状态变更：

| Task 状态 | Run 状态 |
|-----------|---------|
| `completed` | `completed` |
| `failed` / `cancelled` | `failed` |

### 事件发布

状态变更时发布 `autopilot_run:done` WebSocket 事件，前端实时更新 UI。同时上报 analytics 事件用于监控。

## 完整流程图

```
┌─────────────────────────────────────────────────────────────────┐
│  Scheduler (30s ticker)                                          │
│                                                                  │
│  tick → ClaimDueScheduleTriggers (atomic UPDATE, 行级锁)          │
│           │                                                      │
│           ▼                                                      │
│  for each trigger:                                               │
│    ┌─ GetAutopilot()                                             │
│    ├─ DispatchAutopilot()                                        │
│    │    ├─ 准入检查 (agent runtime online?)                       │
│    │    ├─ CreateAutopilotRun                                    │
│    │    ├─ create_issue: 建 issue → EnqueueTaskForIssue          │
│    │    └─ run_only: CreateAutopilotTask → NotifyTaskEnqueued    │
│    └─ advanceNextRun() → ComputeNextRun(cron, tz) → 写回 DB      │
│                                                                  │
└──────────────────────────────────┬──────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────┐
│  Agent Daemon (桌面端 / CLI)                                      │
│                                                                  │
│  WebSocket 收到 wakeup → poll agent_task_queue → 执行任务         │
│  → 更新 task status (completed / failed)                         │
│                                                                  │
└──────────────────────────────────┬──────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────┐
│  Result Sync (事件驱动)                                           │
│                                                                  │
│  SyncRunFromIssue() ← issue 状态变更事件                          │
│  SyncRunFromTask()  ← task 完成/失败事件                          │
│                                                                  │
│  → 更新 autopilot_run.status (completed / failed)                │
│  → 发布 autopilot_run:done WebSocket 事件                        │
│  → 上报 analytics                                                │
└─────────────────────────────────────────────────────────────────┘
```

## 关键设计决策

| 设计 | 原因 |
|------|------|
| **Claim-then-advance** | 先置 NULL 锁定，执行完再算下次时间。PostgreSQL 行级锁保证多实例安全。 |
| **30 秒轮询间隔** | 平衡及时性与 DB 负载。对于分钟级 cron 精度足够。 |
| **准入门控** | 避免离线 agent 堆积无效任务，节省资源。 |
| **崩溃恢复** | 启动时自动修复中断的调度状态，无需人工干预。 |
| **事件驱动同步** | 不轮询结果，靠状态变更事件反向更新，降低延迟和 DB 压力。 |
| **两种执行模式** | 满足不同场景：需要追踪的用 issue，轻量自动化用 run_only。 |

## 已知问题

- **时区硬编码 Bug**（[#3312](https://github.com/multica-ai/multica/issues/3312)）：`buildIssueDescription` 和 `interpolateTemplate` 中时间格式化硬编码为 UTC，未使用 trigger 配置的时区。

## 相关源文件

| 文件 | 职责 |
|------|------|
| `server/cmd/server/autopilot_scheduler.go` | 调度器主循环、claim、advance、崩溃恢复 |
| `server/internal/service/autopilot.go` | Dispatch 逻辑、准入检查、结果同步 |
| `server/internal/service/cron.go` | Cron 表达式解析、时区感知的 next run 计算 |
| `server/internal/handler/autopilot.go` | HTTP API（创建/更新 trigger、手动触发等） |
| `server/pkg/db/queries/autopilot.sql` | 所有调度相关 SQL 查询 |
| `server/migrations/042_autopilot.up.sql` | 表结构定义 |
