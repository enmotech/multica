"use client";

import { useEffect, useRef } from "react";
import type { Agent, AgentTask } from "@multica/core/types";

export type DojoTransitionEvent = {
  agentId: string;
  type: "victory" | "defeat";
  taskId: string;
};

export type DojoFeedEvent = {
  /** Unique id for React key (taskId + event type). */
  id: string;
  agentId: string;
  agentName: string;
  type: "started" | "completed" | "failed";
  /** Short human-readable label derived from trigger_summary or kind. */
  label: string;
  ts: number;
};

/** Derive a short label (≤ 28 chars) from a task. */
function taskLabel(task: AgentTask): string {
  if (task.trigger_summary) {
    const s = task.trigger_summary.trim();
    return s.length <= 28 ? s : `${s.slice(0, 27)}…`;
  }
  switch (task.kind) {
    case "autopilot":    return "autopilot task";
    case "chat":         return "chat task";
    case "quick_create": return "quick create";
    case "direct":       return "direct task";
    default:             return "task";
  }
}

/**
 * Detects VICTORY / DEFEAT transitions by diffing consecutive task snapshots,
 * and emits feed events for task started / completed / failed.
 *
 * Compares the set of running task IDs per agent between renders. When a
 * previously-running task disappears from the snapshot, its terminal status
 * (completed → victory, failed/cancelled → defeat) determines which overlay
 * to trigger. New task IDs entering the running set fire "started" feed events.
 *
 * All callbacks are held in refs so the effect only depends on `snapshot`.
 */
export function useDojotransitions(
  snapshot: readonly AgentTask[] | undefined,
  onTransition: (event: DojoTransitionEvent) => void,
  agents?: readonly Agent[],
  onFeedEvent?: (event: DojoFeedEvent) => void,
): void {
  // Map<agentId, Set<taskId>> — tracks which tasks were running last render.
  const prevRunningRef = useRef<Map<string, Set<string>>>(new Map());
  // Stable refs so the effect doesn't re-run on every render.
  const onTransitionRef = useRef(onTransition);
  onTransitionRef.current = onTransition;
  const agentsRef = useRef(agents);
  agentsRef.current = agents;
  const onFeedEventRef = useRef(onFeedEvent);
  onFeedEventRef.current = onFeedEvent;

  useEffect(() => {
    if (!snapshot) return;

    // Build current running set per agent.
    const currentRunning = new Map<string, Set<string>>();
    for (const task of snapshot) {
      if (task.status === "running") {
        const set = currentRunning.get(task.agent_id);
        if (set) {
          set.add(task.id);
        } else {
          currentRunning.set(task.agent_id, new Set([task.id]));
        }
      }
    }

    // Build a lookup for all tasks in the snapshot by id for status checks.
    const taskById = new Map<string, AgentTask>();
    for (const task of snapshot) {
      taskById.set(task.id, task);
    }

    // Agent name lookup (best-effort; falls back to agentId prefix).
    const agentNameById = new Map<string, string>();
    for (const a of agentsRef.current ?? []) {
      agentNameById.set(a.id, a.name);
    }
    const agentName = (id: string) => agentNameById.get(id) ?? id.slice(0, 8);

    const now = Date.now();

    // Detect tasks that left the running set (completed or failed).
    for (const [agentId, prevIds] of prevRunningRef.current) {
      const currIds = currentRunning.get(agentId) ?? new Set<string>();
      for (const taskId of prevIds) {
        if (currIds.has(taskId)) continue; // Still running — no event.

        const task = taskById.get(taskId);
        if (!task) continue; // Not in snapshot at all — skip.

        if (task.status === "completed") {
          onTransitionRef.current({ agentId, type: "victory", taskId });
          onFeedEventRef.current?.({
            id: `${taskId}-completed`,
            agentId,
            agentName: agentName(agentId),
            type: "completed",
            label: taskLabel(task),
            ts: now,
          });
        } else if (task.status === "failed" || task.status === "cancelled") {
          onTransitionRef.current({ agentId, type: "defeat", taskId });
          onFeedEventRef.current?.({
            id: `${taskId}-failed`,
            agentId,
            agentName: agentName(agentId),
            type: "failed",
            label: taskLabel(task),
            ts: now,
          });
        }
      }
    }

    // Detect tasks that newly entered the running set (started).
    // Skip on the very first diff (prevRunningRef was empty) to avoid a burst
    // of stale "started" events for tasks that were already running before the
    // page was opened.
    if (prevRunningRef.current.size > 0) {
      for (const [agentId, currIds] of currentRunning) {
        const prevIds = prevRunningRef.current.get(agentId) ?? new Set<string>();
        for (const taskId of currIds) {
          if (prevIds.has(taskId)) continue; // Was already running.

          const task = taskById.get(taskId);
          if (!task) continue;

          onFeedEventRef.current?.({
            id: `${taskId}-started`,
            agentId,
            agentName: agentName(agentId),
            type: "started",
            label: taskLabel(task),
            ts: now,
          });
        }
      }
    }

    prevRunningRef.current = currentRunning;
  }, [snapshot]);
}
