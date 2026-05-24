"use client";

import { useCallback, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePresenceMap } from "@multica/core/agents";
import { agentListOptions } from "@multica/core/workspace/queries";
import { agentTaskSnapshotOptions } from "@multica/core/agents/queries";
import { AgentDojoCard } from "./agent-dojo-card";
import { PixelScene } from "./pixel-scene";
import { ActivityFeed } from "./activity-feed";
import { useDojotransitions } from "./use-dojo-transitions";
import { usePictureInPicture } from "./use-picture-in-picture";
import { deriveSteadyState } from "./pixel-frames";
import type { DojoFeedEvent } from "./use-dojo-transitions";

type TransientOverlay = "victory" | "defeat";

/**
 * Position slots for each state area in the room (% of room image).
 *
 * working:     bottom center (red carpet area)
 * waiting:     middle-right (sofa area)
 * sleeping:    top-center (bed area)
 * vacationing: bottom-left (potion room)
 * overloaded:  top-right (kitchen/storage)
 */
const STATE_POSITIONS: Record<string, { x: number; y: number }[]> = {
  working: [
    { x: 35, y: 86 }, { x: 45, y: 86 }, { x: 55, y: 86 }, { x: 65, y: 86 },
    { x: 75, y: 86 }, { x: 85, y: 86 }, { x: 40, y: 92 }, { x: 50, y: 92 },
  ],
  waiting: [
    { x: 54, y: 34 }, { x: 60, y: 34 }, { x: 57, y: 40 },
  ],
  sleeping: [
    { x: 28, y: 16 }, { x: 35, y: 16 }, { x: 31, y: 22 },
  ],
  vacationing: [
    { x: 12, y: 72 }, { x: 20, y: 72 }, { x: 16, y: 78 },
  ],
  overloaded: [
    { x: 72, y: 16 }, { x: 78, y: 16 }, { x: 75, y: 22 },
  ],
};

/**
 * The Agent Dojo page — a pixel-art room showing every agent's
 * live activity state. Agents move between room areas as their state changes.
 */
export function AgentDojoPage() {
  const wsId = useWorkspaceId();
  const { byAgent, loading } = useWorkspacePresenceMap(wsId);

  const { data: agents = [] } = useQuery({
    ...agentListOptions(wsId ?? ""),
    enabled: !!wsId,
  });

  const { data: snapshot } = useQuery({
    ...agentTaskSnapshotOptions(wsId ?? ""),
    enabled: !!wsId,
  });

  const [overlays, setOverlays] = useState<Map<string, { type: TransientOverlay; seq: number }>>(
    new Map(),
  );

  const [feedEvents, setFeedEvents] = useState<readonly DojoFeedEvent[]>([]);

  const handleTransition = useCallback(
    ({ agentId, type }: { agentId: string; type: TransientOverlay }) => {
      setOverlays((prev) => {
        const next = new Map(prev);
        const current = prev.get(agentId);
        next.set(agentId, { type, seq: (current?.seq ?? 0) + 1 });
        return next;
      });
    },
    [],
  );

  const handleFeedEvent = useCallback((event: DojoFeedEvent) => {
    setFeedEvents((prev) => [event, ...prev].slice(0, 20));
  }, []);

  useDojotransitions(snapshot, handleTransition, agents, handleFeedEvent);

  const { isSupported: pipSupported, isOpen: pipOpen, toggle: pipToggle, sceneRef } =
    usePictureInPicture({ width: 520, height: 400 });

  // Group agents by their position state to assign positions
  const agentPositions = new Map<string, { x: number; y: number; z: number }>();
  const stateCounters: Record<string, number> = {};

  for (const agent of agents) {
    const presence = byAgent.get(agent.id);
    if (!presence) continue;
    const key = deriveSteadyState(presence);
    const idx = stateCounters[key] ?? 0;
    stateCounters[key] = idx + 1;
    const positions = STATE_POSITIONS[key] ?? STATE_POSITIONS.sleeping!;
    const pos = positions[idx % positions.length]!;
    // Agents beyond the slot count wrap back to the same base positions but
    // are nudged slightly so they never sit at exactly the same coordinates.
    const wrap = Math.floor(idx / positions.length);
    agentPositions.set(agent.id, {
      x: pos.x + wrap * 3,
      y: pos.y + wrap * 2,
      z: 10 + Math.floor(pos.y + wrap * 2),
    });
  }

  // Header counters
  let working = 0, queued = 0, idle = 0, offline = 0;
  for (const p of byAgent.values()) {
    if (p.availability === "offline" || p.availability === "unstable") { offline++; continue; }
    if (p.workload === "working") working++;
    else if (p.workload === "queued") queued++;
    else idle++;
  }

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <span className="font-mono text-sm text-muted-foreground">Loading dojo…</span>
      </div>
    );
  }

  if (agents.length === 0) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-2">
        <span className="text-2xl">⚔️</span>
        <span className="font-mono text-sm text-muted-foreground">No agents in this workspace yet.</span>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col gap-4 overflow-y-auto bg-background p-6">
      {/* ── Header ── */}
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-bold text-brand" style={{ fontFamily: "var(--font-pixel), monospace" }}>⚔️ Agent Dojo</h1>
        <div className="flex items-center gap-3">
          <div className="flex gap-3 font-mono text-xs text-muted-foreground">
            {working > 0 && <span className="text-success">{working} working</span>}
            {queued > 0  && <span className="text-warning">{queued} queued</span>}
            {idle > 0    && <span>{idle} idle</span>}
            {offline > 0 && <span className="text-muted-foreground/60">{offline} offline</span>}
          </div>
          {pipSupported && (
            <button
              onClick={pipToggle}
              className="flex size-7 items-center justify-center rounded text-base text-muted-foreground hover:bg-accent hover:text-foreground"
              title={pipOpen ? "Close pop-out" : "Pop out"}
              aria-label={pipOpen ? "Close floating dojo window" : "Pop out dojo to a floating window"}
            >
              {pipOpen ? "✕" : "⧉"}
            </button>
          )}
        </div>
      </div>

      {/* ── Scene + feed ── */}
      <div ref={sceneRef} className="flex min-h-0 flex-1 flex-col gap-4 bg-background">
        {/* Keyframes for dojo animations */}
        <style>{`
          @keyframes dojo-zzz {
            0%   { opacity: 1; transform: translateY(0) scale(1); }
            100% { opacity: 0; transform: translateY(-14px) scale(1.3); }
          }
          @keyframes dojo-work-bob {
            0%, 100% { transform: translateY(0); }
            50% { transform: translateY(-3px); }
          }
          @keyframes dojo-sleep-breathe {
            0%, 100% { transform: scaleY(1); }
            50% { transform: scaleY(0.97); }
          }
          @keyframes dojo-overload-shake {
            0%, 100% { transform: translateX(0); }
            25% { transform: translateX(-2px); }
            75% { transform: translateX(2px); }
          }
          @keyframes dojo-victory-jump {
            0%, 100% { transform: translateY(0); }
            50% { transform: translateY(-8px); }
          }
          @keyframes dojo-defeat-fall {
            0% { transform: rotate(0deg); }
            100% { transform: rotate(10deg); }
          }
          @keyframes feed-scroll {
            from { transform: translateX(0); }
            to   { transform: translateX(-50%); }
          }
          .dojo-anim-work { animation: dojo-work-bob 0.8s ease-in-out infinite; }
          .dojo-anim-sleep { animation: dojo-sleep-breathe 2s ease-in-out infinite; opacity: 0.8; }
          .dojo-anim-overload { animation: dojo-overload-shake 0.2s ease-in-out infinite; }
          .dojo-anim-vacation { opacity: 0.5; filter: grayscale(0.5); }
          .dojo-anim-wait { animation: dojo-work-bob 1.5s ease-in-out infinite; }
          .dojo-anim-victory { animation: dojo-victory-jump 0.4s ease-in-out infinite; }
          .dojo-anim-defeat { animation: dojo-defeat-fall 0.5s ease-in-out forwards; }
        `}</style>

        <PixelScene>
          {agents.map((agent) => {
            const presence = byAgent.get(agent.id);
            if (!presence) return null;
            const overlayEntry = overlays.get(agent.id);
            const pos = agentPositions.get(agent.id);
            if (!pos) return null;

            return (
              <div
                key={agent.id}
                className="absolute transition-[left,top] duration-700 ease-in-out"
                style={{
                  left: `${pos.x}%`,
                  top: `${pos.y}%`,
                  zIndex: pos.z,
                }}
              >
                <AgentDojoCard
                  agent={agent}
                  presence={presence}
                  overlay={overlayEntry?.type ?? null}
                />
              </div>
            );
          })}
        </PixelScene>

        {/* Activity feed ticker */}
        <ActivityFeed events={feedEvents} />
      </div>
    </div>
  );
}
