"use client";

import { useEffect, useRef } from "react";
import { FRAME_COLS, FRAME_ROWS, PALETTE, type PixelFrame } from "./pixel-frames";

interface PixelCanvasProps {
  frame: PixelFrame;
  /** Pixels per "pixel" cell. Default 5. */
  scale?: number;
  className?: string;
}

/**
 * Renders a single pixel-art frame onto a <canvas> element.
 * Uses `image-rendering: pixelated` for crisp upscaling.
 */
export function PixelCanvas({ frame, scale = 5, className }: PixelCanvasProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    ctx.clearRect(0, 0, FRAME_COLS * scale, FRAME_ROWS * scale);

    for (let y = 0; y < frame.length; y++) {
      const row = frame[y];
      if (!row) continue;
      for (let x = 0; x < row.length; x++) {
        const char = row[x];
        if (!char) continue;
        const colour = PALETTE[char];
        if (!colour) continue;
        ctx.fillStyle = colour;
        ctx.fillRect(x * scale, y * scale, scale, scale);
      }
    }
  }, [frame, scale]);

  return (
    <canvas
      ref={canvasRef}
      width={FRAME_COLS * scale}
      height={FRAME_ROWS * scale}
      style={{ imageRendering: "pixelated", width: FRAME_COLS * scale, height: FRAME_ROWS * scale }}
      className={className}
    />
  );
}
