# Active Tasks 侧边栏 Widget

> Status: Draft
> Owner: TBD
> Last updated: 2026-05-24

## TL;DR

在侧边栏增加一个 **Active** 折叠区域，实时展示当前 workspace 内所有正在执行（running / queued）的 agent 任务。有任务时自动出现，任务全部结束后自动消失。底层数据基础设施已经完备，MVP 为纯前端改动。

---

## 背景与动机

目前"agent 正在工作"的状态只在两处可见：

1. **issue 详情页** — `AgentLiveCard`，sticky banner，信息最完整，但必须打开具体 issue 才能看到。
2. **squad / agent 视图** — `AgentLivePeekCard`，悬浮在 agent 头像上才触发，被动感知。

当用户在 issues 列表、projects、或其他页面工作时，无法感知到后台有 agent 任务在运行。侧边栏 widget 提供一个**始终可见的全局入口**，让用户随时知道"谁在做什么"，并一键跳转到对应 issue。

---

## UX 设计

### 布局方案

在侧边栏 `Pinned` 和 `Workspace` 两个 section 之间插入一个新的 **Active** 折叠区，与 Pinned section 使用完全相同的 `Collapsible` + `SidebarGroup` 容器形态。

```
┌─────────────────────────────────┐
│ M  MoClaw                    ∨  │
├─────────────────────────────────┤
│ 🔍 Search...          Ctrl+K    │
│ ✏  New Task                 C   │
├─────────────────────────────────┤
│ Inbox                        7  │
│ My Tasks                        │
├─────────────────────────────────┤
│ Active  ●  ∨              2     │  ← 有任务时出现，● 为脉冲绿点
│  🤖 Kimi    MOC-20  running     │  ← 点击跳转到 issue 详情页
│  🤖 Claude  MOC-18  queued      │
├─────────────────────────────────┤
│ Pinned  ∨                       │
│  📁 mig-oracle-pg               │
│  📁 postgresql-upgrade          │
├─────────────────────────────────┤
│ Workspace                       │
│  Tasks / Projects / ...         │
└─────────────────────────────────┘
```

### 行为规则

Active section **始终渲染**，不随任务有无而出现或消失。这与 Pinned section 的逻辑不同——Pinned 是用户主动策划的列表，空列表没有展示价值；Active 是系统驱动的 feed，始终可见能帮助用户建立"agent 活动在这里出现"的心智模型。

| 场景 | 表现 |
|---|---|
| 无 active 任务 | Section 保持可见，内容区显示空状态提示（见下） |
| 有 ≥1 个任务 | 任务行列表展示，section label 显示脉冲绿点 + 数量 |
| 新任务触发 | 对应行实时插入（WS 事件驱动） |
| 任务结束 | 对应行实时消失（WS 事件驱动） |
| 所有任务结束 | 任务行清空，回到空状态，脉冲点消失 |
| 点击任务行 | 跳转到该 issue 的详情页 |
| issue_id 为空 | 该行不可点击，无跳转（autopilot / chat 触发的任务） |

**空状态文案**：一行浅色提示，例如 "No agents are working right now"，样式与 sidebar 其他空状态保持一致（`text-muted-foreground text-xs px-2`）。

### 每行信息

```
[agent 头像]  [agent 名]  [issue identifier]  [状态点]
    🤖          Kimi         MOC-20              ●
```

- **状态点颜色**：`running` → 绿色脉冲；`queued` / `dispatched` → 橙色静态点
- issue identifier 来自 snapshot query join issue cache；无需额外 fetch（见下方数据层说明）
- agent 名过长时截断，与 Pinned 行的文字截断样式保持一致

---

## 数据层

### 现有基础设施（无需新增）

**`agentTaskSnapshotOptions(wsId)`**（`packages/core/agents/queries.ts`）

- 返回 workspace 内所有 active 任务 + 每个 agent 最近一次 terminal 任务
- `staleTime: 30s`，主要 freshness 来自 WS task 事件 invalidation
- `task:dispatch`、`task:running`、`task:completed`、`task:failed`、`task:cancelled` 均已在 `useRealtimeSync` 中触发 invalidation

前端只需：
1. 消费该 query
2. 过滤 `status === "running" | "queued" | "dispatched"`
3. join agent list（已在缓存）获取 agent 名和头像
4. 通过 `issue_id` 从 issue detail 缓存读取 `identifier`（如 `MOC-20`）

### issue identifier 获取策略

`AgentTask.issue_id` 是 UUID，identifier（`MOC-20`）存在于 issue detail query 缓存中。

MVP 策略：**读缓存，不主动 fetch**。

- 用 `queryClient.getQueryData(issueDetailOptions(wsId, issue_id).queryKey)` 读缓存
- 缓存命中 → 显示 identifier；未命中 → 显示缩略 UUID 或 "—"，待用户访问 issue 页后自动补全
- 避免 N 个 active 任务触发 N 个额外请求

> 如果后续觉得 identifier 显示率太低，可在 backend 的 `getAgentTaskSnapshot` 响应里直接带上 `issue_identifier` 字段，一次 backend 改动解决，不影响 MVP。

---

## 实现计划

### MVP 涉及文件

**新建**

```
packages/views/layout/active-tasks-sidebar-section.tsx   ← 新组件，~100 行
```

**修改**

```
packages/views/layout/app-sidebar.tsx                    ← 插入新 section，+5 行左右
```

不需要改动：后端、API client、query 定义、WS 逻辑。

### 组件结构（伪代码）

```tsx
// active-tasks-sidebar-section.tsx
export function ActiveTasksSidebarSection({ wsId }: { wsId: string }) {
  const { data: snapshot = [] } = useQuery(agentTaskSnapshotOptions(wsId));
  const { data: agents = [] } = useQuery(agentListOptions(wsId));

  const activeTasks = snapshot.filter(
    (t) => t.status === "running" || t.status === "queued" || t.status === "dispatched"
  );

  // Section is always rendered — never conditionally hidden.
  // This teaches users that agent activity surfaces here,
  // even when no tasks are currently running.
  return (
    <Collapsible defaultOpen>
      <SidebarGroup>
        <SidebarGroupLabel render={<CollapsibleTrigger />}>
          {activeTasks.length > 0 && <PulsingDot />}
          <span>{t("Active")}</span>
          {activeTasks.length > 0 && <span>{activeTasks.length}</span>}
        </SidebarGroupLabel>
        <CollapsibleContent>
          <SidebarGroupContent>
            {activeTasks.length === 0 ? (
              <p className="px-2 text-xs text-muted-foreground">
                {t("No agents are working right now")}
              </p>
            ) : (
              <SidebarMenu>
                {activeTasks.map((task) => (
                  <ActiveTaskRow key={task.id} task={task} agents={agents} wsId={wsId} />
                ))}
              </SidebarMenu>
            )}
          </SidebarGroupContent>
        </CollapsibleContent>
      </SidebarGroup>
    </Collapsible>
  );
}
```

### app-sidebar.tsx 改动位置

在 Pinned section 之前插入：

```tsx
{wsId && <ActiveTasksSidebarSection wsId={wsId} />}
```

---

## MVP 范围

**包含**
- Active section 始终可见，空状态显示提示文案
- running / queued / dispatched 任务行展示
- agent 头像 + 名字 + issue identifier（读缓存）+ 状态点
- 点击跳转到 issue 详情页
- WS 实时驱动（复用现有 invalidation 链路）
- issue_id 为空时行不可点击

**不包含（后续迭代）**
- 任务结束时的短暂"完成驻留"动效（绿色 ✓ 短暂停留后消失）
- 主动 fetch 缺失的 issue identifier
- backend snapshot 接口带 issue_identifier 字段
- Stop 按钮（已在 issue 详情页的 AgentLiveCard 上提供）
- 多任务折叠（同一 agent 有多个任务时合并为一行）

---

## 后续迭代考虑

### 任务结束动效

任务行"直接消失"在用户视觉上较突兀。可在组件本地维护一个"即将消失"状态：

```
running → 完成（✓ 绿色，显示约 1.5s）→ fade out → 行移除
```

需要在组件内用 `useEffect` + `setTimeout` 维护一个本地 `fadingOut` Set，snapshot query 本身不提供这个过渡态。实现独立，不影响 MVP 数据流。

### Backend snapshot 携带 issue_identifier

当前 MVP 靠读 TQ 缓存获取 identifier，命中率依赖用户是否访问过该 issue。如需保证 100% 显示，可在 `getAgentTaskSnapshot` 的 SQL 查询中 JOIN `issues` 表带出 `identifier` 字段，成本极低且对 snapshot 消费方无破坏性影响。
