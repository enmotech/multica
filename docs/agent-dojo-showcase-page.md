# Agent Dojo — Live Showcase Page

> Status: Implemented (branch: kamusis/issue3)
> Owner: TBD
> Last updated: 2026-05-24

## TL;DR

A standalone "brag screen" page where every agent in the workspace is rendered as an animated pixel-art character. Character animations are driven purely by existing real-time data (`agentTaskSnapshotOptions` + `agentListOptions` + WS invalidation). Zero new backend work. Think Tamagotchi meets ops dashboard.

---

## Concept

**"The Dojo"** — a pixel-art scene where agents live at their workstations. Their animation reflects exactly what they're doing right now:

- **Working** → furiously typing at a glowing terminal
- **Queued** → standing in line, tapping foot, hourglass floating overhead
- **Offline / idle** → slumped at desk, `z z z` drifting upward
- **Task just completed** → jumps up, arms raised, star burst (1.5 s, then back to idle/working)
- **Task just failed** → slumps, red `×` appears (1.5 s, then back)

---

## Page Layout

```
┌──────────────────────────────────────────────────────────────────┐
│  ⚔️  Agent Dojo         MoClaw workspace      [3 working  2 idle] │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ╔════════════════════ pixel scene ═══════════════════════════╗  │
│  ║  🌆 tiled floor  |  pixel window  |  ambient glow          ║  │
│  ║                                                            ║  │
│  ║  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐  ║  │
│  ║  │ 🖥️[▓▓] │ │  [zzz] │ │ [⌛]  │ │ 🖥️[▓▓] │ │ [zzz] │  ║  │
│  ║  │ [🤖💨] │ │  [😴]  │ │  [🤖] │ │ [🤖💨] │ │  [😴]  │  ║  │
│  ║  │ ══════ │ │ ══════ │ │ ══════ │ │ ══════ │ │ ══════ │  ║  │
│  ║  │  Kimi  │ │ Claude │ │  GPT  │ │ Llama │ │Mistral│  ║  │
│  ║  │RUNNING │ │  IDLE  │ │QUEUED │ │RUNNING│ │ IDLE  │  ║  │
│  ║  │ MOC-20 │ │   —    │ │MOC-18 │ │MOC-22 │ │   —   │  ║  │
│  ║  └────────┘ └────────┘ └────────┘ └────────┘ └────────┘  ║  │
│  ╚════════════════════════════════════════════════════════════╝  │
│                                                                  │
│  ── Activity Feed ─────────────────────────────────────────────  │
│  ● Kimi started MOC-20  ·  ✓ Claude completed MOC-15  ·  ...    │
└──────────────────────────────────────────────────────────────────┘
```

---

## Character Animation State Machine

Each agent card renders a pixel-art character driven by a local state machine:

```
AgentAvailability + Workload
         │
         ▼
┌────────────────────────────────────────────────┐
│  offline / unstable  →  VACATIONING            │
│  online + idle       →  SLEEPING               │
│  online + queued     →  WAITING                │
│  online + working (1 task)   →  WORKING         │
│  online + working (≥2 tasks) →  OVERLOADED     │
└────────────────────────────────────────────────┘
         +
  Transient overlays (local component state):
  ┌──────────────────────────────────────────┐
  │  task just completed  →  VICTORY  (1.5s) │
  │  task just failed     →  DEFEAT   (1.5s) │
  └──────────────────────────────────────────┘
```

Transient overlays are driven by comparing the previous snapshot with the new one: if a task transitions from `running → completed`, trigger VICTORY; `running/queued → failed/cancelled` triggers DEFEAT. After the timeout, the character reverts to the steady state.

**State semantics:**
- **VACATIONING** — runtime unreachable (`offline` or `unstable`). Agent is completely absent, like being on holiday. Card shows "runtime offline" subtitle below the state label. Both `offline` and `unstable` share this state — no intermediate visual.
- **SLEEPING** — runtime online but nothing to do (`idle`). Agent is at their workstation, dozing. Instantly available when a task arrives.

---

## Pixel Character Designs

Characters are 16 × 24 pixel grids rendered via `<canvas>` (one char per pixel in a string array, drawn as filled rectangles). Two animation frames per state are enough for a convincing walk-cycle / typing effect.

### WORKING (2 frames, alternating every 200 ms)

Agent is actively executing a task — could be database ops, code generation, API calls, or any workload. Character is at the workstation with arms shifting between frames (busy motion).

```
Frame A                Frame B
. . H H H . .         . . H H H . .
. H O O O H .         . H O O O H .
. . H W H . .         . . H W H . .   ← W = eye open, blinking on frame B
. . H H H . .         . . H - H . .   ← - = closed eye
. . H . H . .         . H . . . H .   ← arms shift
. H H H H H .         . H H H H H .
. . K K K . .         . . K K K . .   ← K = workstation surface
```

Legend: `H`=body colour, `O`=face, `W`=eye open, `-`=eye closed, `K`=workstation surface

Ambient effect: cursor blink on the pixel terminal (`█` blinking at 500 ms) above the workstation.

### WAITING (2 frames, alternating every 600 ms)

Slight foot-tap: bottom pixel shifts left/right between frames. Hourglass floats overhead using a CSS `translateY` keyframe (`-4px → 0 → -4px`, 2 s infinite).

### SLEEPING (3 frames)

Character slumped with head on desk. `z` `Z` `Z` drift upward (CSS `@keyframes` moving three staggered `<text>` elements up and fading out).

### VACATIONING (static or 2 frames)

Agent is offline / unreachable. Scene replaces the workstation entirely: pixel beach with sand, a small sun umbrella, and the character lying underneath wearing sunglasses. Card subtitle shows **"runtime offline"** in muted text below the state label. `offline` and `unstable` both map here — no separate intermediate state.

### VICTORY (4-frame one-shot sequence, ~400 ms each)

1. Character crouches (arms down)
2. Jump (character y-offset −8 px)
3. Arms raised, star burst (`*` particles radiate outward via Framer Motion)
4. Land, settle → fade to WORKING or SLEEPING

### DEFEAT (3-frame one-shot)

1. Character droops
2. Head hits desk (horizontal shift)
3. Red `×` appears floating above, fades out over 1 s → settle to SLEEPING

---

## Technical Implementation

### Pixel Character Renderer

```tsx
// packages/views/dojo/pixel-canvas.tsx
// Actual implementation uses <canvas> instead of SVG for lower DOM overhead.

type PixelFrame = readonly string[];  // each string = one row, one char = one pixel

// PALETTE maps single chars to hex colors (null = transparent).
// Full palette defined in pixel-frames.ts.
const PALETTE: Record<string, string | null> = {
  H: "#6366f1",  // indigo body
  O: "#fde68a",  // face
  W: "#1e1b4b",  // eye
  ".": null,     // transparent
};

export function PixelCanvas({ frame, scale = 5 }: { frame: PixelFrame; scale?: number }) {
  const ref = useRef<HTMLCanvasElement>(null);
  useEffect(() => {
    const ctx = ref.current?.getContext("2d");
    if (!ctx) return;
    ctx.clearRect(0, 0, FRAME_COLS * scale, FRAME_ROWS * scale);
    frame.forEach((row, y) => {
      [...row].forEach((ch, x) => {
        const color = PALETTE[ch];
        if (!color) return;
        ctx.fillStyle = color;
        ctx.fillRect(x * scale, y * scale, scale, scale);
      });
    });
  }, [frame, scale]);
  return (
    <canvas
      ref={ref}
      width={FRAME_COLS * scale}
      height={FRAME_ROWS * scale}
      style={{ imageRendering: "pixelated" }}
    />
  );
}
```

### Agent Card Component

```tsx
// packages/views/dojo/agent-dojo-card.tsx

export function AgentDojoCard({ agent, presence }: { agent: Agent; presence: AgentPresenceDetail }) {
  const [animState, setAnimState] = useState<AnimState>(deriveAnimState(presence));
  const prevPresenceRef = useRef(presence);
  const [frameIdx, setFrameIdx] = useState(0);

  // Detect transient transitions
  useEffect(() => {
    const prev = prevPresenceRef.current;
    if (prev.workload === "working" && presence.workload !== "working") {
      // task left the running state — check if it was a completion or failure
      // (caller passes recentTerminal to distinguish)
      triggerOverlay("victory");  // or "defeat"
    }
    prevPresenceRef.current = presence;
  }, [presence]);

  // Frame ticker
  useEffect(() => {
    const ms = FRAME_MS[animState];
    const id = setInterval(() => setFrameIdx(i => (i + 1) % FRAMES[animState].length), ms);
    return () => clearInterval(id);
  }, [animState]);

  return (
    <motion.div layout className="flex flex-col items-center gap-1">
      <PixelTerminal active={animState === "coding"} />
      <PixelSprite frame={FRAMES[animState][frameIdx]} />
      <PixelDesk />
      <span className="font-mono text-xs">{agent.name}</span>
      <StatusBadge state={animState} issueIdentifier={...} />
    </motion.div>
  );
}
```

### Data Flow

```
agentTaskSnapshotOptions(wsId)   agentListOptions(wsId)
           │                              │
           └──────────────┬───────────────┘
                          ▼
              useAgentPresence(wsId)   ← already exists in packages/core
                          │
                          ▼
                AgentDojoCard (per agent)
                          │
                          ▼
               local animState state machine
```

No new queries, no new WS subscriptions. Everything piggybacks on existing infrastructure.

### Scene & Ambient Effects

| Effect | Implementation |
|---|---|
| Tiled pixel floor | `repeating-conic-gradient` checkerboard, pinned to scene bottom |
| Pixel window (day/night) | SVG pixel art with sun/moon swap based on local time (`useIsNight`) |
| Window glow | `@keyframes dojo-window-glow` opacity pulse (4 s cycle) |
| CRT scanlines overlay | `repeating-linear-gradient` at 8% opacity, `pointer-events-none` |
| Activity feed ticker | Scrolling ticker of recent WS-driven task events |
| Typing sound (opt-in) | `AudioContext` playing a short click sample on frame change (future) |

### Completion / Failure particle burst

Use Framer Motion's `AnimatePresence` with a set of 8 `motion.div` star particles, each animating to a random `(x, y)` offset radially:

```tsx
const particles = Array.from({ length: 8 }, (_, i) => ({
  angle: (i / 8) * 2 * Math.PI,
  x: Math.cos(angle) * 40,
  y: Math.sin(angle) * 40,
}));
```

---

## Files to Create / Modify

**New files (all in `packages/views/dojo/`)**

```
packages/views/dojo/agent-dojo-page.tsx          ← page root, grid layout, header counters
packages/views/dojo/agent-dojo-card.tsx          ← single agent card + state machine
packages/views/dojo/pixel-canvas.tsx             ← Canvas pixel renderer (replaces SVG approach)
packages/views/dojo/pixel-frames.ts              ← palette, frame data for all 7 states
packages/views/dojo/pixel-scene.tsx              ← background scene (floor, window, CRT scanlines)
packages/views/dojo/activity-feed.tsx            ← scrolling activity ticker
packages/views/dojo/use-dojo-transitions.ts      ← snapshot task-ID set diff → transient events
```

**App-level wiring**

```
apps/web/app/[workspaceSlug]/(dashboard)/dojo/page.tsx    ← Next.js route shell
```

**No changes to:**
- `packages/core/` (zero new queries or stores)
- `server/` (backend untouched)
- Any existing view component

---

## MVP Scope

**Included**
- All 7 animation states: 5 steady (WORKING, WAITING, SLEEPING, VACATIONING, OVERLOADED) + 2 transient (VICTORY, DEFEAT)
- VICTORY and DEFEAT one-shot overlays driven by per-agent task-ID set diff
- Pixel scene background: checkerboard floor, pixel window with day/night cycle, CRT scanlines
- Activity feed ticker (last N WS-driven task events)
- Header with live counters (X working · Y queued · Z idle · N offline)
- Agent sort: activity group (working → queued → idle → offline), alphabetical within group
- Adaptive card scale: ≤ 12 agents → scale 5 (80 × 120 px), > 12 agents → scale 4 (64 × 96 px)
- Route accessible at `/[workspaceSlug]/dojo`
- Sidebar footer ⚔️ icon with tooltip

**Out of scope (later)**
- Document Picture-in-Picture pop-out mode (tracked in #4)
- Customisable character colour per agent
- "Overheating" effect when agent has capacity-filling tasks for > N minutes
- Fullscreen / presentation mode
- Typing sound (opt-in toggle)

---

## Decisions

### 1. Entry point — sidebar footer icon

Add a single icon button (⚔️) in the sidebar footer alongside Settings and other low-frequency
entries. No text label; tooltip on hover reads "Agent Dojo". Clicking navigates to
`/[wsId]/dojo`.

Rationale: this is a brag screen, not a daily workflow surface. A footer icon is discoverable
without competing with primary navigation. Direct-URL-only is too hidden to serve the
"show off" purpose.

### 2. Agent ordering — activity group, then alphabetical within group

Sort order: `working → queued → sleeping/idle → vacationing/offline`

Within each group, agents are sorted alphabetically by name (stable). Layout re-orders only
when an agent crosses a group boundary, not on every heartbeat. Framer Motion `layout`
animation smooths cross-group transitions.

Rationale: pure workload sorting causes continuous layout thrashing as states flip. Pure
alphabetical buries the busiest agents. Grouped + stable name order gives the best of both.

### 3. Large agent counts — wrapping grid + adaptive card scale

- Grid wraps naturally; rows fill to 5 cards then wrap. Page scrolls vertically if needed.
- **≤ 12 agents**: pixel scale = 5 (character renders at 80 × 120 px)
- **> 12 agents**: pixel scale = 4 (character renders at 64 × 96 px)
- All agents are always shown; no truncation.

A future "Fullscreen / presentation mode" (already in Out of scope) can enforce a Top-8
fixed layout with maximum scale for projector/TV use.
