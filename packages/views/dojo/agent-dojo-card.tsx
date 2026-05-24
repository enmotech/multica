"use client";

import { memo, useEffect, useRef, useState } from "react";
import { motion } from "framer-motion";
import type { AgentPresenceDetail } from "@multica/core/agents";
import type { Agent } from "@multica/core/types";
import { OVERLAY_DURATION_MS, STATE_CONFIG, type AnimState } from "./pixel-frames";
import { PixelCanvas } from "./pixel-canvas";

/** Pixels per cell at different scale breakpoints. */
const SCALE_NORMAL = 5;
const SCALE_COMPACT = 4;

interface AgentDojoCardProps {
  agent: Agent;
  presence: AgentPresenceDetail;
  /** Transient overlay to show (set by the parent via useDojotransitions). */
  overlay: "victory" | "defeat" | null;
  /** Compact mode when the workspace has > 12 agents. */
  compact?: boolean;
}

/** Derives the steady-state animation from presence data. */
function deriveSteadyState(
  p: AgentPresenceDetail,
): Exclude<AnimState, "victory" | "defeat"> {
  if (p.availability === "offline" || p.availability === "unstable") {
    return "vacationing";
  }
  if (p.workload === "queued") return "waiting";
  if (p.workload === "working") return p.runningCount >= 2 ? "overloaded" : "working";
  return "sleeping";
}

/** Label shown below the state name for each steady state. */
const STATE_LABEL: Record<AnimState, string> = {
  working:     "WORKING",
  waiting:     "WAITING",
  sleeping:    "SLEEPING",
  vacationing: "VACATIONING",
  overloaded:  "OVERLOADED",
  victory:     "VICTORY",
  defeat:      "DEFEAT",
};

/**
 * Renders a single agent's pixel-art card in the Dojo.
 *
 * Steady-state animation is derived from AgentPresenceDetail. VICTORY /
 * DEFEAT overlays are driven by the parent (via useDojotransitions) so that
 * the parent can correlate snapshot diffs across all agents in one pass.
 */
export const AgentDojoCard = memo(function AgentDojoCard({
  agent,
  presence,
  overlay,
  compact = false,
}: AgentDojoCardProps) {
  const scale = compact ? SCALE_COMPACT : SCALE_NORMAL;
  const steadyState = deriveSteadyState(presence);

  // Active animation state: steady state unless an overlay is active.
  const [animState, setAnimState] = useState<AnimState>(steadyState);
  const overlayTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Always-current presence snapshot for use inside setTimeout callbacks,
  // avoiding the stale-closure problem when `overlay` triggers but `presence`
  // has since changed.
  const latestPresenceRef = useRef(presence);
  latestPresenceRef.current = presence;

  // Update steady state when presence changes (but not during an overlay).
  useEffect(() => {
    if (animState !== "victory" && animState !== "defeat") {
      setAnimState(steadyState);
    }
  }, [steadyState, animState]);

  // Trigger overlay when parent signals a transition.
  useEffect(() => {
    if (!overlay) return;
    setAnimState(overlay);
    if (overlayTimerRef.current) clearTimeout(overlayTimerRef.current);
    overlayTimerRef.current = setTimeout(() => {
      setAnimState(deriveSteadyState(latestPresenceRef.current));
    }, OVERLAY_DURATION_MS);
    return () => {
      if (overlayTimerRef.current) clearTimeout(overlayTimerRef.current);
    };
  }, [overlay]);

  // Frame ticker: cycle through the current state's frames.
  const [frameIdx, setFrameIdx] = useState(0);
  useEffect(() => {
    setFrameIdx(0); // Reset frame index when state changes.
    const { frames, intervalMs } = STATE_CONFIG[animState];
    if (frames.length <= 1) return;
    const id = setInterval(
      () => setFrameIdx((i) => (i + 1) % frames.length),
      intervalMs,
    );
    return () => clearInterval(id);
  }, [animState]);

  const { frames } = STATE_CONFIG[animState];
  const currentFrame = frames[frameIdx % frames.length]!;

  const isVacationing = animState === "vacationing";
  const isSleeping = animState === "sleeping";

  return (
    <motion.div
      layout
      className="flex flex-col items-center gap-1 rounded-md border border-border bg-card px-3 py-2"
      style={{ width: compact ? 100 : 120 }}
    >
      {/* State label */}
      <span className="font-mono text-[9px] font-bold uppercase tracking-wider text-brand">
        {STATE_LABEL[animState]}
      </span>

      {/* "runtime offline" subtitle for vacationing */}
      {isVacationing && (
        <span className="font-mono text-[8px] uppercase tracking-widest text-destructive">
          runtime offline
        </span>
      )}

      {/* Sprite */}
      <div className="relative">
        <PixelCanvas frame={currentFrame} scale={scale} />

        {/* zzz bubbles for sleeping */}
        {isSleeping && (
          <div className="pointer-events-none absolute right-0 top-2 select-none">
            {(["z", "Z", "Z"] as const).map((ch, i) => (
              <span
                key={i}
                className="absolute text-brand"
                style={{
                  right: 2 + i * 10,
                  top: 15 + i * 6,
                  fontSize: 10 + i * 3,
                  animation: `dojo-zzz 2.2s ease-out ${i * 0.7}s infinite`,
                }}
              >
                {ch}
              </span>
            ))}
          </div>
        )}
      </div>

      {/* Agent name */}
      <span
        className="max-w-full truncate font-mono text-[9px] text-muted-foreground"
        title={agent.name}
      >
        {agent.name}
      </span>
    </motion.div>
  );
});
