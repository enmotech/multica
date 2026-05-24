"use client";

import { useEffect, useState, type ReactNode } from "react";

// Night: 20:00 – 5:59 (8 pm → 6 am)
function checkIsNight(): boolean {
  const h = new Date().getHours();
  return h < 6 || h >= 20;
}

/**
 * Returns whether it is currently night time, polling once per minute.
 * Safe for SSR: initializer runs only on the client (component is "use client").
 */
function useIsNight(): boolean {
  // Initialize to false to avoid SSR/client hydration mismatch — the server
  // has no reliable concept of "local time". The correct value is set on the
  // first client effect, before the first paint with visible content.
  const [isNight, setIsNight] = useState(false);
  useEffect(() => {
    setIsNight(checkIsNight());
    const id = setInterval(() => setIsNight(checkIsNight()), 60_000);
    return () => clearInterval(id);
  }, []);
  return isNight;
}

interface PixelSceneProps {
  /** Agent cards rendered inside the scene grid. */
  children: ReactNode;
}

/**
 * Wraps the agent card grid in a pixel-art "dojo room" scene.
 *
 * Layout (pure CSS/SVG — no data dependencies):
 *  - CRT scanline overlay (absolute, pointer-events-none)
 *  - Pixel window decoration (top-right corner)
 *  - Scrollable agent grid slot (children)
 *  - Checkerboard floor strip (always pinned to bottom)
 */
export function PixelScene({ children }: PixelSceneProps) {
  return (
    <div className="relative flex min-h-0 flex-1 flex-col">
      {/* CRT scanlines: semi-transparent 1px stripe every 4px */}
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0 z-10"
        style={{
          backgroundImage:
            "repeating-linear-gradient(0deg, transparent 0px, transparent 3px, rgb(0 0 0 / 0.08) 3px, rgb(0 0 0 / 0.08) 4px)",
        }}
      />

      {/* Pixel window — top-right corner decoration */}
      <div aria-hidden className="flex justify-end pb-1">
        <PixelWindow />
      </div>

      {/* Agent grid — scrolls independently so the floor always stays visible */}
      <div className="flex min-h-0 flex-1 flex-wrap content-start justify-center gap-3 overflow-y-auto pb-2">
        {children}
      </div>

      {/* Checkerboard floor — pinned to the bottom of the scene */}
      <PixelFloor />
    </div>
  );
}

/**
 * Pixel-art window that switches between a daytime and night-time scene
 * based on the browser's local time (day = 06:00–19:59, night = 20:00–05:59).
 * The glow keyframe (dojo-window-glow) is defined in AgentDojoPage's <style>.
 */
function PixelWindow() {
  const isNight = useIsNight();

  // Sky and glow colors differ between day and night; celestial body is conditional.
  const skyColor  = isNight ? "#1a2744" : "#87ceeb";
  const glowColor = isNight ? "#60a5fa" : "#fbbf24";

  return (
    <svg
      width={60}
      height={50}
      viewBox="0 0 12 10"
      style={{ imageRendering: "pixelated" }}
    >
      {/* Window frame — uses theme border color */}
      <rect x={0} y={0} width={12} height={10} fill="var(--color-border)" />
      {/* Sky panes — pixel-art content, intentionally hardcoded */}
      <rect x={1} y={1} width={4} height={3} fill={skyColor} />
      <rect x={7} y={1} width={4} height={3} fill={skyColor} />
      <rect x={1} y={6} width={4} height={3} fill={skyColor} />
      <rect x={7} y={6} width={4} height={3} fill={skyColor} />
      {/* Celestial body */}
      {isNight ? (
        // Two-pixel crescent moon (upper-right pane)
        <>
          <rect x={9} y={2} width={1} height={1} fill="#fde68a" />
          <rect x={8} y={3} width={1} height={1} fill="#fde68a" />
        </>
      ) : (
        // Cross-shaped pixel sun (upper-right pane)
        <>
          <rect x={9} y={1} width={1} height={1} fill="#fbbf24" />
          <rect x={8} y={2} width={3} height={1} fill="#fbbf24" />
          <rect x={9} y={3} width={1} height={1} fill="#fbbf24" />
        </>
      )}
      {/* Cross dividers — uses theme border color */}
      <rect x={5} y={1} width={2} height={8} fill="var(--color-border)" />
      <rect x={1} y={4} width={10} height={2} fill="var(--color-border)" />
      {/* Breathing glow — same keyframe, color drives day vs night feel */}
      <rect
        x={1}
        y={1}
        width={10}
        height={8}
        fill={glowColor}
        style={{ animation: "dojo-window-glow 4s ease-in-out infinite" }}
      />
    </svg>
  );
}

/** Checkerboard floor strip using a repeating-conic-gradient tile. */
function PixelFloor() {
  return (
    <div
      aria-hidden
      className="h-3 w-full"
      style={{
        backgroundColor: "var(--color-muted)",
        backgroundImage:
          "repeating-conic-gradient(rgb(0 0 0 / 0.1) 0deg 90deg, transparent 90deg 180deg)",
        backgroundSize: "8px 8px",
      }}
    />
  );
}
