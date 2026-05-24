"use client";

import type { DojoFeedEvent } from "./use-dojo-transitions";

interface ActivityFeedProps {
  events: readonly DojoFeedEvent[];
}

/** Icon + color for each event type. */
const EVENT_CONFIG: Record<
  DojoFeedEvent["type"],
  { icon: string; className: string }
> = {
  started:   { icon: "▶", className: "text-brand" },
  completed: { icon: "✓", className: "text-success" },
  failed:    { icon: "✗", className: "text-destructive" },
};

/**
 * Scrolling marquee ticker that shows recent agent task events.
 *
 * With ≥ 4 events: renders two copies side-by-side so the CSS `feed-scroll`
 * keyframe (defined in AgentDojoPage's <style>) can loop seamlessly by
 * translating -50%.
 *
 * With < 4 events: shows a static list to avoid a short strip of text racing
 * across a large blank gap.
 *
 * Hidden when there are no events yet.
 */
export function ActivityFeed({ events }: ActivityFeedProps) {
  if (events.length === 0) return null;

  const shouldScroll = events.length >= 4;

  return (
    <div
      className="overflow-hidden border-t border-border"
      aria-label="Activity feed"
    >
      {shouldScroll ? (
        /* Scrolling: two identical copies to allow seamless loop */
        <div
          className="flex whitespace-nowrap"
          style={{ animation: "feed-scroll 20s linear infinite" }}
        >
          <FeedItems events={events} />
          <FeedItems events={events} aria-hidden />
        </div>
      ) : (
        /* Static: single copy, no animation */
        <FeedItems events={events} />
      )}
    </div>
  );
}

/** One copy of all feed event chips. */
function FeedItems({
  events,
  "aria-hidden": ariaHidden,
}: {
  events: readonly DojoFeedEvent[];
  "aria-hidden"?: true;
}) {
  return (
    <span className="inline-flex gap-4 px-4 py-1" aria-hidden={ariaHidden}>
      {events.map((ev) => {
        const cfg = EVENT_CONFIG[ev.type];
        return (
          <span key={ev.id} className="inline-flex items-center gap-1 font-mono text-[10px]">
            <span className={cfg.className}>{cfg.icon}</span>
            <span className="text-muted-foreground font-semibold">{ev.agentName}</span>
            <span className="text-muted-foreground/70">{ev.label}</span>
          </span>
        );
      })}
    </span>
  );
}
