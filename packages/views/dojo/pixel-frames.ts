/**
 * Shared constants, types, and utilities for Agent Dojo animations.
 */

import type { AgentPresenceDetail } from "@multica/core/agents";

export type AnimState =
  | "working"
  | "waiting"
  | "sleeping"
  | "vacationing"
  | "overloaded"
  | "victory"
  | "defeat";

/** Duration a transient overlay (VICTORY / DEFEAT) stays visible. */
export const OVERLAY_DURATION_MS = 1500;

/**
 * Derives the steady-state animation key from an agent's presence data.
 * Used by both AgentDojoCard (animation) and AgentDojoPage (room positioning)
 * to guarantee the two always agree on the active state.
 */
export function deriveSteadyState(
  p: AgentPresenceDetail,
): Exclude<AnimState, "victory" | "defeat"> {
  if (p.availability === "offline" || p.availability === "unstable") return "vacationing";
  if (p.workload === "queued") return "waiting";
  if (p.workload === "working") return p.runningCount >= 2 ? "overloaded" : "working";
  return "sleeping";
}
