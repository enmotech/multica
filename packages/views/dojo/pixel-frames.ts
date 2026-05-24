/**
 * Shared constants and types for Agent Dojo animations.
 */

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
