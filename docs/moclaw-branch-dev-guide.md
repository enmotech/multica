# MoClaw Branch Development Guide

This guide covers everything you need to develop on the `moclaw` branch — what the branch is,
how it diverges from upstream, environment setup, and the day-to-day workflow.

---

## 1. Branch Overview

The `moclaw` branch is **enmotech's long-running fork** of the upstream `multica-ai/multica` project.
It lives at `origin` (enmotech/multica) and is never pushed to `upstream` (multica-ai/multica).

### Key divergences from upstream `main`

| Area | Change |
|---|---|
| **Brand** | User-facing name renamed from *Multica* → *MoClaw* (UI strings, locales, favicon) |
| **Terminology** | *Issue* → *Task* throughout frontend routing and UI labels |
| **Landing page** | Home page hidden; users land directly in the app |
| **Email** | Provider abstraction added: `resend` (default) or `smtp` (new) |
| **Sidebar** | Active Tasks section showing running agent tasks |

### Cherry-picked from upstream

Commits that exist in upstream `main` but were selectively applied to `moclaw` before the
official upstream sync. These are **not** moclaw-exclusive — they came from upstream PRs.

| Commit | Upstream PR | Description |
|---|---|---|
| `c5f334d9` | [#2686](https://github.com/multica-ai/multica/pull/2686) | Multi-select bulk import in "Copy from runtime" panel + Redis-backed local skills store |

---

## 2. Repository Layout

```
origin    → https://github.com/enmotech/multica.git   (your fork)
upstream  → https://github.com/multica-ai/multica.git  (upstream)
```

Active branches:

| Branch | Purpose |
|---|---|
| `main` | Mirror of upstream — **never commit directly here** |
| `moclaw` | Main development branch for MoClaw customizations |
| `your-feature` | Short-lived feature branches cut from `moclaw` |

---

## 3. Environment Setup

### 3.1 Prerequisites

- Go 1.26.1+
- Node 22+ with pnpm
- Docker (for PostgreSQL)

### 3.2 First-time setup

```bash
git clone https://github.com/enmotech/multica.git
cd multica
git checkout moclaw

# Add upstream remote
git remote add upstream https://github.com/multica-ai/multica.git

# One-command setup: creates DB, runs migrations, installs deps
make dev
```

### 3.3 Environment variables (moclaw-specific additions)

Copy `.env.example` to `.env` and configure the following fields that are **new in moclaw**
(not present in upstream):

```bash
# Email provider: "resend" (default) or "smtp"
EMAIL_PROVIDER=resend

# If using Resend:
RESEND_API_KEY=re_xxxx

# If using SMTP instead:
# EMAIL_PROVIDER=smtp
# SMTP_HOST=smtp.gmail.com
# SMTP_PORT=587
# SMTP_USER=you@gmail.com
# SMTP_PASSWORD=your-app-password
# SMTP_TLS=true
```

> **Local dev tip:** If neither `RESEND_API_KEY` nor SMTP is configured, verification codes
> print to stdout — no email service needed.

---

## 4. Daily Development Workflow

### 4.1 Branch rules

```
main      ← upstream only. NEVER commit here.
moclaw    ← base for all MoClaw work.
feature/  ← cut from moclaw, merge back into moclaw via PR.
```

If you develop with Claude Code, a branch-guard hook can enforce this automatically —
any `Write`/`Edit` attempt on `main` is blocked with a reminder to switch branches.
See [§10 Developing with Claude Code — Protecting `main`](#10-developing-with-claude-code--protecting-main) for setup instructions.

### 4.2 Starting a new feature

```bash
# Make sure moclaw is up to date
git checkout moclaw
git pull origin moclaw

# Cut a feature branch
git checkout -b feature/your-feature-name

# ... do your work ...

# Push and open a PR → moclaw (not main)
git push -u origin feature/your-feature-name
```

### 4.3 Syncing `main` with upstream

Do this periodically to keep `main` current. It does **not** affect `moclaw`.

```bash
git fetch upstream
git checkout main
git merge upstream/main   # always fast-forward; main has no local commits
git push origin main
git checkout moclaw       # switch back
```

---

## 5. Running the App

```bash
# Everything at once (recommended)
make dev

# Separately
make server          # Go backend on :8080
pnpm dev:web         # Next.js on :3000
pnpm dev:desktop     # Electron with HMR
```

---

## 6. Testing

```bash
make check           # Full suite: typecheck + TS tests + Go tests + E2E

# Targeted
pnpm typecheck       # TypeScript only
pnpm test            # Vitest (all packages)
make test            # Go tests
```

### Running a single test

```bash
# TypeScript (views package example)
pnpm --filter @multica/views exec vitest run skills/components/runtime-local-skill-import-panel.test.tsx

# Go
cd server && go test ./internal/handler/ -run TestDaemon
```

---

## 7. MoClaw-specific Code Areas

When working on MoClaw features, these are the primary files to know:

### Brand / terminology

- `packages/views/locales/en/` and `packages/views/locales/zh-Hans/` — all UI strings
- Search for `"task"` vs `"issue"` — MoClaw uses "task" throughout

### Routing

- `apps/web/app/[workspaceSlug]/(dashboard)/tasks/` — task routes (MoClaw addition)
- `apps/desktop/src/renderer/src/routes.tsx` — desktop routes

### Email

- `server/internal/service/mail_provider.go` — `Provider` interface
- `server/internal/service/mail_smtp.go` — SMTP implementation
- `server/internal/service/mail_resend.go` — Resend implementation
- `server/internal/service/email.go` — orchestrator

### Sidebar / Active Tasks

- `packages/views/layout/active-tasks-sidebar-section.tsx` — the widget
- `packages/views/layout/app-sidebar.tsx` — wired in here

---

## 8. Committing

Follow conventional commits format, scoped to the affected area:

```
feat(tasks): add bulk status update
fix(email): handle SMTP connection timeout
refactor(brand): update remaining Multica references
```

Types: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`, `perf`

---

## 9. Opening a Pull Request

PRs should target **`moclaw`**, not `main`.

```bash
gh pr create \
  --base moclaw \
  --title "feat(tasks): your feature" \
  --body "..."
```

---

## 10. Developing with Claude Code — Protecting `main`

### Branch policy

`main` exists **only** as a read-only mirror of upstream. Its sole purpose is to let you
study what upstream has fixed or shipped:

```bash
git log main..upstream/main --oneline   # what upstream added since last sync
git diff main upstream/main -- server/  # how upstream changed the Go server
```

**Nothing is ever committed to `main` directly.** All development happens on `moclaw` or a
feature branch that eventually merges into `moclaw`.

```
upstream/main  ──►  main  (sync only, read-only)
                      │
                      └── moclaw  ◄──  feature/xxx  (all development here)
```

### Why Claude Code can accidentally write to `main`

When you ask Claude to implement something, it writes files using the `Edit` / `Write` tools.
If you happen to be on `main` at that moment — e.g. after syncing upstream and forgetting to
switch back — Claude will silently modify files on `main`, which breaks the policy above.

### Setting up a branch-guard hook

Claude Code supports `PreToolUse` hooks: shell commands that run **before** each tool call
and can block it by returning a deny decision. The following hook checks the current branch
and rejects any `Edit` / `Write` / `NotebookEdit` call when you are on `main`.

#### Step 1 — Decide the scope

| File | Scope |
|---|---|
| `~/.claude/settings.json` | Global — protects `main` in every project |
| `.claude/settings.local.json` | This project only, not committed to git |

For a rule this fundamental, **global** (`~/.claude/settings.json`) is recommended.

#### Step 2 — Read the existing file before editing

Always read first to avoid overwriting other settings:

```bash
cat ~/.claude/settings.json
```

#### Step 3 — Add the hook

Merge the `"hooks"` key into your existing `~/.claude/settings.json`:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Write|Edit|NotebookEdit",
        "hooks": [
          {
            "type": "command",
            "command": "branch=$(git branch --show-current 2>/dev/null); if [ \"$branch\" = \"main\" ]; then printf '{\"hookSpecificOutput\":{\"hookEventName\":\"PreToolUse\",\"permissionDecision\":\"deny\",\"permissionDecisionReason\":\"Blocked: code changes are not allowed on the main branch. Switch to moclaw or create a feature branch first.\"}}'; fi",
            "statusMessage": "Checking branch guard..."
          }
        ]
      }
    ]
  }
}
```

**How it works:**

| Field | Purpose |
|---|---|
| `PreToolUse` | Fires before the tool executes — can block it entirely |
| `matcher` | Which tools to intercept (`Write`, `Edit`, `NotebookEdit`) |
| `type: "command"` | Run a shell command |
| `permissionDecision: "deny"` | Reject the tool call and show the reason to Claude |
| `statusMessage` | Text shown in the spinner while the hook runs |

#### Step 4 — Pipe-test the command before saving

Simulate both cases manually to confirm the logic is correct:

```bash
# Should produce NO output (current branch is not main → allowed)
branch=$(git branch --show-current)
if [ "$branch" = "main" ]; then
  printf '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"Blocked: switch to moclaw first."}}'
fi

# Simulate main → should print the deny JSON
branch="main"
if [ "$branch" = "main" ]; then
  printf '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"Blocked: switch to moclaw first."}}'
fi
```

#### Step 5 — Validate JSON syntax

```bash
jq -e '.hooks.PreToolUse[] | select(.matcher == "Write|Edit|NotebookEdit") | .hooks[] | .command' \
  ~/.claude/settings.json
```

Exit 0 and the command string is printed → the JSON is valid and the hook is reachable.

#### Step 6 — Reload the config

Hooks are loaded at session start. In a running session, open `/hooks` in Claude Code to
reload without restarting.

### What the protection looks like in practice

When Claude tries to edit a file while on `main`, it receives:

```
Blocked: code changes are not allowed on the main branch.
Switch to moclaw or create a feature branch first.
```

Claude will then stop, remind you to switch branches, and wait for your instruction.
The file is never touched.

### PR target reminder

When opening pull requests, always set the base branch to `moclaw`:

```bash
gh pr create \
  --base moclaw \
  --title "feat(tasks): your feature" \
  --body "..."
```

GitHub's default base branch for this repo is `main`. **Always override it explicitly.**
A PR merged into `main` by accident cannot be easily undone without rewriting history.

---

## 11. Quick Reference

```bash
# Where am I?
git branch --show-current

# What's different from upstream?
git log main..moclaw --oneline

# Sync main with upstream (safe, no effect on moclaw)
git fetch upstream && git checkout main && git merge upstream/main && git push origin main && git checkout moclaw

# Start a feature
git checkout moclaw && git pull && git checkout -b feature/name

# Full check before pushing
make check
```
