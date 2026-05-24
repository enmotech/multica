"use client";

import type { ReactNode } from "react";

interface PixelSceneProps {
  /** Agent elements rendered at state-based positions. */
  children: ReactNode;
}

/**
 * Pixel-art room scene using a pre-rendered tilemap background.
 * Agents are absolutely positioned on top of the room image.
 */
export function PixelScene({ children }: PixelSceneProps) {
  return (
    <div className="relative min-h-[200px] flex-1 overflow-hidden rounded-lg border border-border">
      {/* CRT scanlines */}
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0 z-20"
        style={{
          backgroundImage:
            "repeating-linear-gradient(0deg, transparent 0px, transparent 3px, rgb(0 0 0 / 0.03) 3px, rgb(0 0 0 / 0.03) 4px)",
        }}
      />

      {/* Room background */}
      <img
        src="/dojo/room.png"
        alt=""
        aria-hidden
        className="block w-full h-auto"
        style={{ imageRendering: "pixelated" }}
      />

      {/* Agent overlay layer */}
      <div className="absolute inset-0 z-10">
        {children}
      </div>
    </div>
  );
}
