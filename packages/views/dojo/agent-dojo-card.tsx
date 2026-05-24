"use client";

import { memo, useEffect, useRef, useState } from "react";
import type { AgentPresenceDetail } from "@multica/core/agents";
import type { Agent } from "@multica/core/types";
import { deriveSteadyState, OVERLAY_DURATION_MS, type AnimState } from "./pixel-frames";

interface AgentDojoCardProps {
  agent: Agent;
  presence: AgentPresenceDetail;
  /** Transient overlay to show (set by the parent via useDojotransitions). */
  overlay: "victory" | "defeat" | null;
}

/** CSS class for the sprite animation based on state. */
function spriteAnimClass(state: AnimState): string {
  switch (state) {
    case "working":     return "dojo-anim-work";
    case "overloaded":  return "dojo-anim-overload";
    case "sleeping":    return "dojo-anim-sleep";
    case "vacationing": return "dojo-anim-vacation";
    case "waiting":     return "dojo-anim-wait";
    case "victory":     return "dojo-anim-victory";
    case "defeat":      return "dojo-anim-defeat";
  }
}

/** Status dot color class. Intentionally uses hardcoded colors for pixel-art
 *  decorative styling — these are game UI elements, not semantic app states. */
function dotClass(state: AnimState): string {
  switch (state) {
    case "working":
    case "victory":
      return "bg-green-400 shadow-[0_0_4px_#4ade80]";
    case "overloaded":
    case "defeat":
      return "bg-red-400 shadow-[0_0_4px_#ef4444]";
    case "waiting":
      return "bg-yellow-400 shadow-[0_0_4px_#fbbf24]";
    case "sleeping":
      return "bg-indigo-400";
    case "vacationing":
      return "bg-gray-500";
  }
}

/**
 * Renders a single agent in the Dojo room using the tileset sprite.
 */
export const AgentDojoCard = memo(function AgentDojoCard({
  agent,
  presence,
  overlay,
}: AgentDojoCardProps) {
  const steadyState = deriveSteadyState(presence);

  const [animState, setAnimState] = useState<AnimState>(steadyState);
  const overlayTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
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

  const isSleeping = animState === "sleeping";

  return (
    <div className="flex flex-col items-center gap-0">
      {/* Agent sprite */}
      <div className="relative">
        <img
          src="/dojo/sprite.png"
          alt={agent.name}
          className={`h-[40px] w-[24px] ${spriteAnimClass(animState)}`}
          style={{ imageRendering: "pixelated" }}
        />

        {/* zzz bubbles for sleeping */}
        {isSleeping && (
          <div className="pointer-events-none absolute right-[-8px] top-[-5px] select-none font-bold text-brand">
            <span className="absolute" style={{ right: 0, top: 0, fontSize: 8, animation: "dojo-zzz 2.2s ease-out 0s infinite" }}>z</span>
            <span className="absolute" style={{ right: 6, top: -4, fontSize: 10, animation: "dojo-zzz 2.2s ease-out 0.6s infinite" }}>z</span>
            <span className="absolute" style={{ right: 12, top: -8, fontSize: 12, animation: "dojo-zzz 2.2s ease-out 1.2s infinite" }}>Z</span>
          </div>
        )}
      </div>

      {/* Name label */}
      <div className="mt-0.5 flex items-center gap-1 rounded bg-black/75 px-1 py-px border border-white/10">
        <span className={`inline-block size-[5px] rounded-full ${dotClass(animState)}`} />
        <span className="max-w-[80px] truncate font-mono text-[7px] text-white" title={agent.name}>
          {agent.name}
        </span>
      </div>
    </div>
  );
});
