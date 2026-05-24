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
import type { DojoFeedEvent } from "./use-dojo-transitions";
import type { AgentPresenceDetail } from "@multica/core/agents";

type TransientOverlay = "victory" | "defeat";

/** Sorts agents by activity group, then alphabetically within each group. */
function sortAgents<T extends { id: string; name: string }>(
  agents: T[],
  byAgent: Map<string, AgentPresenceDetail>,
): T[] {
  function groupOrder(p: AgentPresenceDetail | undefined): number {
    if (!p || p.availability === "offline" || p.availability === "unstable") return 3;
    if (p.workload === "working") return 0;
    if (p.workload === "queued") return 1;
    return 2; // idle / sleeping
  }
  return [...agents].sort((a, b) => {
    const ga = groupOrder(byAgent.get(a.id));
    const gb = groupOrder(byAgent.get(b.id));
    if (ga !== gb) return ga - gb;
    return a.name.localeCompare(b.name);
  });
}

/**
 * The Agent Dojo page — a pixel-art "brag screen" showing every agent's
 * live activity state. Zero new queries: piggybacks on existing
 * agentListOptions + agentTaskSnapshotOptions infrastructure.
 */
export function AgentDojoPage() {
  const wsId = useWorkspaceId();
  const { byAgent, loading } = useWorkspacePresenceMap(wsId);

  // Raw agent list (for ordering and rendering cards).
  const { data: agents = [] } = useQuery({
    ...agentListOptions(wsId ?? ""),
    enabled: !!wsId,
  });

  // Raw snapshot — needed by useDojotransitions to diff task ID sets.
  const { data: snapshot } = useQuery({
    ...agentTaskSnapshotOptions(wsId ?? ""),
    enabled: !!wsId,
  });

  // Per-agent transient overlay: "victory" | "defeat" | null.
  // Using a counter as value so re-triggering the same event for the same
  // agent re-mounts the overlay (e.g. two consecutive failures).
  const [overlays, setOverlays] = useState<Map<string, { type: TransientOverlay; seq: number }>>(
    new Map(),
  );

  // Activity feed ring buffer — newest first, max 20 entries.
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

  const sortedAgents = sortAgents(agents, byAgent);
  const compact = agents.length > 12;

  // Header counters.
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
        <h1 className="font-mono text-lg font-bold text-brand">⚔️ Agent Dojo</h1>
        <div className="flex items-center gap-3">
          <div className="flex gap-3 font-mono text-xs text-muted-foreground">
            {working > 0 && <span className="text-success">{working} working</span>}
            {queued > 0  && <span className="text-warning">{queued} queued</span>}
            {idle > 0    && <span>{idle} idle</span>}
            {offline > 0 && <span className="text-muted-foreground/60">{offline} offline</span>}
          </div>
          {/* Pop-out button — only rendered when Document PiP is supported (Chrome 116+) */}
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

      {/* ── Scene + feed: wrapped so both move into the PiP window together ── */}
      <div ref={sceneRef} className="flex min-h-0 flex-1 flex-col gap-4 bg-background">
        {/* Keyframes travel with the scene so they work inside the PiP window */}
        <style>{`
          @keyframes dojo-zzz {
            0%   { opacity: 1; transform: translateY(0) scale(1); }
            100% { opacity: 0; transform: translateY(-28px) scale(1.3); }
          }
          @keyframes dojo-window-glow {
            0%, 100% { opacity: 0; }
            50%       { opacity: 0.08; }
          }
          @keyframes feed-scroll {
            from { transform: translateX(0); }
            to   { transform: translateX(-50%); }
          }
        `}</style>
        {/* Agent grid inside the pixel scene */}
        <PixelScene>
          {sortedAgents.map((agent) => {
            const presence = byAgent.get(agent.id);
            if (!presence) return null;
            const overlayEntry = overlays.get(agent.id);
            return (
              <AgentDojoCard
                key={agent.id}
                agent={agent}
                presence={presence}
                overlay={overlayEntry?.type ?? null}
                compact={compact}
              />
            );
          })}
        </PixelScene>

        {/* Activity feed ticker */}
        <ActivityFeed events={feedEvents} />
      </div>
    </div>
  );
}
