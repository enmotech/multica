# MoClaw PostgreSQL SQL Patterns

本文档总结了 MoClaw 项目中操作 PostgreSQL 数据库的所有 SQL 模式。基于 `server/pkg/db/queries/` 下 29 个 sqlc 文件及 1 个动态 SQL handler 的完整分析。

## 概览

- **ORM**: 无，使用 [sqlc](https://sqlc.dev/) 从 SQL 生成类型安全的 Go 代码
- **驱动**: pgx/v5
- **Schema 管理**: 增量 migration 文件（`server/migrations/`）
- **查询文件**: `server/pkg/db/queries/*.sql`（29 个文件）
- **动态 SQL**: 仅搜索功能在 Go handler 中拼接

---

## 1. 基础 CRUD

```sql
-- 查询单条
SELECT * FROM table WHERE id = $1;

-- 查询多条
SELECT * FROM table WHERE workspace_id = $1 ORDER BY created_at DESC;

-- 插入并返回
INSERT INTO table (col1, col2, ...) VALUES ($1, $2, ...) RETURNING *;

-- 更新并返回
UPDATE table SET col1 = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- 删除
DELETE FROM table WHERE id = $1;
```

---

## 2. 条件过滤

### 可选参数过滤（sqlc.narg 模式）

```sql
WHERE workspace_id = $1
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('priority')::text IS NULL OR priority = sqlc.narg('priority'))
```

### IN / NOT IN 列表

```sql
WHERE status IN ('queued', 'dispatched', 'running')
WHERE status NOT IN ('done', 'cancelled')
```

### ANY 数组匹配

```sql
WHERE assignee_id = ANY(sqlc.narg('assignee_ids')::uuid[])
WHERE id = ANY(@ids::uuid[])
WHERE project_id = ANY(sqlc.arg('project_ids')::uuid[])
```

### NULL 检查

```sql
WHERE issue_id IS NULL
WHERE archived_at IS NULL
WHERE session_id IS NOT NULL
```

### 时间范围

```sql
WHERE expires_at > now()
WHERE created_at > now() - INTERVAL '30 days'
WHERE last_seen_at < now() - make_interval(secs => @stale_seconds::double precision)
```

### ILIKE 模糊匹配

```sql
AND COALESCE(error, '') ILIKE '%400%'
AND COALESCE(error, '') ILIKE '%invalid_request_error%'
```

---

## 3. 排序与分页

```sql
-- 单列排序
ORDER BY created_at DESC

-- 多列排序
ORDER BY priority DESC, created_at ASC
ORDER BY position ASC, created_at DESC

-- 表达式排序
ORDER BY LOWER(name) ASC

-- COALESCE 排序（多个时间字段取最新）
ORDER BY COALESCE(completed_at, started_at, dispatched_at, created_at) DESC

-- NULLS LAST
ORDER BY completed_at DESC NULLS LAST

-- 分页
LIMIT $2 OFFSET $3

-- 取最新一条
ORDER BY created_at DESC LIMIT 1
```

---

## 4. JOIN

```sql
-- INNER JOIN
SELECT m.*, u.name as user_name, u.email as user_email
FROM member m
JOIN "user" u ON u.id = m.user_id
WHERE m.workspace_id = $1;

-- LEFT JOIN
SELECT i.*, iss.status as issue_status
FROM inbox_item i
LEFT JOIN issue iss ON iss.id = i.issue_id
WHERE ...;

-- 多表 JOIN
SELECT wi.*, w.name AS workspace_name, u.name AS inviter_name
FROM workspace_invitation wi
JOIN workspace w ON w.id = wi.workspace_id
JOIN "user" u ON u.id = wi.inviter_id
WHERE ...;

-- JOIN + 聚合
SELECT atq.agent_id, tu.model, SUM(tu.input_tokens)::bigint AS input_tokens
FROM task_usage tu
JOIN agent_task_queue atq ON atq.id = tu.task_id
WHERE ...
GROUP BY atq.agent_id, tu.model;
```

---

## 5. 聚合函数

```sql
-- COUNT
SELECT count(*) FROM issue WHERE workspace_id = $1;
COUNT(*)::int AS run_count
COUNT(*)::bigint AS total_count

-- COUNT DISTINCT
COUNT(DISTINCT tu.task_id)::int AS task_count

-- SUM
SUM(tu.input_tokens)::bigint AS input_tokens

-- COALESCE + SUM（防 NULL）
COALESCE(SUM(tu.input_tokens), 0)::bigint AS total_input_tokens

-- MAX
COALESCE(MAX(position), 0)::float8 AS max_position

-- COUNT FILTER（条件计数，PostgreSQL 特有）
COUNT(*) FILTER (WHERE status IN ('done', 'cancelled'))::bigint AS done_count
COUNT(*) FILTER (WHERE status = 'failed')::int AS failed_count
```

---

## 6. GROUP BY

```sql
-- 单列
GROUP BY project_id

-- 多列
GROUP BY DATE(tu.created_at), tu.provider, tu.model

-- 表达式
GROUP BY EXTRACT(HOUR FROM tu.created_at), tu.model

-- JSONB 字段
GROUP BY details->>'to_type', details->>'to_id'
```

---

## 7. 子查询

### EXISTS / NOT EXISTS

```sql
-- 存在性检查
AND NOT EXISTS (
    SELECT 1 FROM agent_task_queue active
    WHERE active.agent_id = atq.agent_id
      AND active.status IN ('dispatched', 'running')
      AND active.issue_id = atq.issue_id
)

-- SELECT EXISTS 返回布尔
SELECT EXISTS(
    SELECT 1 FROM issue_subscriber
    WHERE issue_id = $1 AND user_type = $2 AND user_id = $3
) AS subscribed;
```

### IN 子查询

```sql
WHERE runtime_id IN (
    SELECT id FROM agent_runtime WHERE status = 'offline'
)
```

### NOT IN 子查询

```sql
AND id NOT IN (SELECT DISTINCT runtime_id FROM agent)
```

### INSERT ... SELECT WHERE EXISTS（带守卫的插入）

```sql
INSERT INTO issue_to_label (issue_id, label_id)
SELECT sqlc.arg('issue_id')::uuid, sqlc.arg('label_id')::uuid
WHERE EXISTS (
    SELECT 1 FROM issue i WHERE i.id = ... AND i.workspace_id = ...
)
AND EXISTS (
    SELECT 1 FROM issue_label l WHERE l.id = ... AND l.workspace_id = ...
)
ON CONFLICT DO NOTHING;
```

### 标量子查询

```sql
INSERT INTO chat_session (..., runtime_id)
VALUES ($1, $2, $3, $4, (SELECT runtime_id FROM agent WHERE id = $2));
```

---

## 8. CTE (WITH 子句)

### 批量过期 + FOR UPDATE SKIP LOCKED

```sql
WITH victims AS (
    SELECT id FROM agent_task_queue
    WHERE status = 'queued'
      AND created_at < now() - make_interval(secs => @ttl_secs::double precision)
    ORDER BY created_at ASC
    LIMIT @max_per_tick::int
    FOR UPDATE SKIP LOCKED
)
UPDATE agent_task_queue t
SET status = 'failed', completed_at = now(), error = 'task expired in queue'
FROM victims v
WHERE t.id = v.id
  AND t.status = 'queued'
RETURNING t.*;
```

### CTE 聚合 + JOIN（故障率统计）

```sql
WITH stats AS (
    SELECT autopilot_id,
           count(*) FILTER (WHERE status IN ('completed', 'failed')) AS total,
           count(*) FILTER (WHERE status = 'failed') AS failed
    FROM autopilot_run
    WHERE created_at >= sqlc.arg('since')::timestamptz
    GROUP BY autopilot_id
)
SELECT a.id, a.workspace_id, s.total::bigint, s.failed::bigint
FROM autopilot a
JOIN stats s ON s.autopilot_id = a.id
WHERE a.status = 'active'
  AND s.total >= sqlc.arg('min_runs')::bigint
  AND s.failed::float8 / NULLIF(s.total, 0)::float8 >= sqlc.arg('fail_ratio_threshold')::float8;
```

---

## 9. UNION ALL

```sql
-- 合并活跃任务 + 每个 agent 最近一条完成/失败任务
SELECT atq.* FROM agent_task_queue atq
JOIN agent a ON a.id = atq.agent_id
WHERE a.workspace_id = $1
  AND atq.status IN ('queued', 'dispatched', 'running')

UNION ALL

SELECT t.* FROM (
  SELECT DISTINCT ON (atq.agent_id) atq.*
  FROM agent_task_queue atq
  JOIN agent a ON a.id = atq.agent_id
  WHERE a.workspace_id = $1
    AND atq.status IN ('completed', 'failed')
  ORDER BY atq.agent_id, atq.completed_at DESC NULLS LAST
) t;
```

---

## 10. DISTINCT ON（PostgreSQL 特有）

```sql
-- 每个 agent 取最近一条完成的任务
SELECT DISTINCT ON (atq.agent_id) atq.*
FROM agent_task_queue atq
JOIN agent a ON a.id = atq.agent_id
WHERE a.workspace_id = $1 AND atq.status IN ('completed', 'failed')
ORDER BY atq.agent_id, atq.completed_at DESC NULLS LAST;
```

---

## 11. 时间与日期函数

```sql
-- 当前时间
now()

-- 时间截断
DATE_TRUNC('day', atq.completed_at)::timestamptz AS bucket

-- 提取小时
EXTRACT(HOUR FROM started_at)::int AS hour

-- 转为日期
DATE(tu.created_at) AS date

-- 动态间隔
make_interval(secs => @stale_seconds::double precision)

-- 固定间隔
now() - interval '1 hour'
now() - INTERVAL '30 days'
```

---

## 12. UPSERT (ON CONFLICT)

### DO UPDATE SET

```sql
INSERT INTO task_usage (task_id, provider, model, input_tokens, ...)
VALUES ($1, $2, $3, $4, ...)
ON CONFLICT (task_id, provider, model)
DO UPDATE SET
    input_tokens = EXCLUDED.input_tokens,
    output_tokens = EXCLUDED.output_tokens,
    updated_at = now();
```

### DO UPDATE 幂等（保留原值）

```sql
INSERT INTO issue_reaction (issue_id, workspace_id, actor_type, actor_id, emoji)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (issue_id, actor_type, actor_id, emoji)
DO UPDATE SET created_at = issue_reaction.created_at
RETURNING *;
```

### DO NOTHING

```sql
INSERT INTO agent_skill (agent_id, skill_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;
```

---

## 13. 条件更新（COALESCE partial update）

```sql
-- 只更新传入的非 NULL 字段，其余保持原值
UPDATE agent SET
    name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    runtime_config = COALESCE(sqlc.narg('runtime_config'), runtime_config),
    updated_at = now()
WHERE id = $1
RETURNING *;
```

---

## 14. CASE 表达式

### UPDATE 中的条件赋值

```sql
UPDATE agent AS a
SET status = CASE WHEN EXISTS (
    SELECT 1 FROM agent_task_queue q
    WHERE q.agent_id = a.id AND q.status IN ('dispatched', 'running')
) THEN 'working' ELSE 'idle' END,
    updated_at = now()
WHERE a.id = $1 RETURNING *;
```

### 动态排序中的 CASE（搜索排名）

```sql
ORDER BY CASE
    WHEN i.number = $N THEN 0
    WHEN LOWER(i.title) = $1 THEN 1
    WHEN LOWER(i.title) LIKE $1 || '%' THEN 2
    WHEN LOWER(i.title) LIKE '%' || $1 || '%' THEN 3
    ELSE 7
END
```

### 状态优先级排序

```sql
CASE i.status
    WHEN 'in_progress' THEN 0
    WHEN 'in_review' THEN 1
    WHEN 'todo' THEN 2
    WHEN 'blocked' THEN 3
    WHEN 'backlog' THEN 4
    WHEN 'done' THEN 5
    WHEN 'cancelled' THEN 6
    ELSE 7
END
```

---

## 15. 锁与并发控制

### FOR UPDATE SKIP LOCKED（任务队列 claim）

```sql
-- 原子性地 claim 下一个可用任务，跳过已被锁定的行
SELECT atq.id FROM agent_task_queue atq
WHERE atq.agent_id = $1 AND atq.status = 'queued'
  AND NOT EXISTS (...)
ORDER BY atq.priority DESC, atq.created_at ASC
LIMIT 1
FOR UPDATE SKIP LOCKED
```

### FOR UPDATE（悲观锁保护删除）

```sql
-- 获取排他锁，防止并发 INSERT 引用该行
SELECT id FROM chat_session WHERE id = $1 FOR UPDATE;
```

### xmax trick（判断 INSERT vs UPDATE）

```sql
-- PostgreSQL 内部字段，xmax=0 表示新插入的行
RETURNING *, (xmax = 0) AS inserted;
```

---

## 16. JSONB 操作

```sql
-- 存储
settings JSONB NOT NULL DEFAULT '{}'
context JSONB

-- 箭头操作符提取文本
details->>'to_type' AS assignee_type
details->>'to_id' AS assignee_id

-- 作为 GROUP BY 键
GROUP BY details->>'to_type', details->>'to_id'
```

---

## 17. 类型转换

```sql
::uuid          -- UUID
::uuid[]        -- UUID 数组
::text          -- 文本
::boolean       -- 布尔
::int           -- 整数
::bigint        -- 大整数
::float8        -- 双精度浮点
::timestamptz   -- 带时区时间戳
::double precision  -- 双精度
```

---

## 18. 动态 SQL（Go handler 中构建）

仅用于 issue 搜索功能（`internal/handler/issue.go`）：

```sql
-- LIKE 模糊搜索（大小写不敏感）
LOWER(i.title) LIKE '%' || $1 || '%'

-- 多词 AND 搜索：每个 term 都必须出现
(LOWER(i.title) LIKE '%' || $2 || '%' OR LOWER(COALESCE(i.description,'')) LIKE '%' || $2 || '%')
AND (LOWER(i.title) LIKE '%' || $3 || '%' OR ...)

-- escapeLike 防注入（转义 %, _, \）
```

---

## 19. 跨表 UPDATE（FROM 子句）

```sql
-- 从另一张表获取条件进行更新
UPDATE autopilot_trigger t
SET next_run_at = NULL
FROM autopilot a
WHERE t.autopilot_id = a.id
  AND t.kind = 'schedule'
  AND a.status = 'active'
RETURNING t.*, a.workspace_id;
```

---

## 20. 其他常用 Pattern

### RETURNING 控制

```sql
RETURNING *                    -- 返回完整行
RETURNING id                   -- 仅返回 ID
RETURNING token_hash           -- 返回特定字段
RETURNING id, workspace_id     -- 返回多个字段
```

### 布尔存在性检查

```sql
SELECT count(*) > 0 AS has_active FROM agent_task_queue
WHERE issue_id = $1 AND status IN ('queued', 'dispatched', 'running');
```

### NULLIF 防除零

```sql
s.failed::float8 / NULLIF(s.total, 0)::float8 >= @threshold
```

### 计算列 / 类型转换列

```sql
SELECT cs.*, (cs.unread_since IS NOT NULL)::bool AS has_unread
FROM chat_session cs WHERE ...;
```

### 原子性条件更新（幂等）

```sql
-- 只在首次执行时生效
UPDATE issue SET first_executed_at = now()
WHERE id = $1 AND first_executed_at IS NULL
RETURNING ...;
```

### INSERT ... SELECT FROM（克隆行）

```sql
INSERT INTO agent_task_queue (agent_id, runtime_id, ...)
SELECT p.agent_id, p.runtime_id, ..., p.attempt + 1
FROM agent_task_queue p
WHERE p.id = $1
RETURNING *;
```

---

## 总结特征

| 维度 | 评估 |
|------|------|
| 复杂度 | 中高（CTE、UNION ALL、DISTINCT ON、FOR UPDATE SKIP LOCKED） |
| 并发控制 | 重度（任务队列是核心场景，大量行级锁） |
| 聚合分析 | 中度（token usage 统计、活动趋势、故障率计算） |
| 全文搜索 | LIKE-based（未用 tsvector/tsquery，未用 pgvector） |
| JSONB | 轻度（存储配置/详情，偶尔提取字段做 GROUP BY） |
| 窗口函数 | 未使用（用 DISTINCT ON 替代） |
| 递归 CTE | 未使用 |
| 存储过程/触发器 | 未使用（纯 SQL 查询 + Go 业务逻辑） |
| PostgreSQL 特有语法 | 重度（DISTINCT ON、FILTER、xmax、make_interval、::类型转换） |
