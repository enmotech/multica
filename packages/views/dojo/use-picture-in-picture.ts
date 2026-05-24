"use client";

import { useCallback, useEffect, useRef, useState } from "react";

// Type declarations for the Document Picture-in-Picture API (Chrome 116+).
// Not yet in the standard TypeScript lib.
declare global {
  interface Window {
    documentPictureInPicture?: {
      requestWindow(options?: {
        width?: number;
        height?: number;
        disallowReturnToOpener?: boolean;
        preferInitialWindowPlacement?: boolean;
      }): Promise<Window>;
      readonly window: Window | null;
    };
  }
}

/** True when the browser supports Document Picture-in-Picture (Chrome 116+). */
export function isDocumentPiPSupported(): boolean {
  return typeof window !== "undefined" && "documentPictureInPicture" in window;
}

/**
 * Manages Document Picture-in-Picture for a DOM subtree.
 *
 * Call `toggle()` to move the element referenced by `sceneRef` into a
 * floating always-on-top window. All same-origin stylesheets are cloned into
 * the PiP window so Tailwind classes and CSS theme tokens render correctly.
 * The dark/light mode class is copied from `<html>` so the theme matches.
 *
 * When the PiP window closes (user or programmatic), the DOM subtree is
 * moved back to its original position — React's component tree is
 * unaffected because only the DOM nodes are re-parented, not the fiber tree.
 *
 * Cleans up automatically if the host component unmounts while PiP is open.
 */
export function usePictureInPicture({ width = 480, height = 360 } = {}) {
  const sceneRef = useRef<HTMLDivElement>(null);
  const pipWindowRef = useRef<Window | null>(null);
  const [isOpen, setIsOpen] = useState(false);
  const [pending, setPending] = useState(false);
  const isSupported = isDocumentPiPSupported();

  // Close PiP window if the host component unmounts while it's open.
  useEffect(() => {
    return () => {
      pipWindowRef.current?.close();
    };
  }, []);

  const toggle = useCallback(async () => {
    if (!isSupported || pending) return;

    // Close if already open.
    if (pipWindowRef.current) {
      pipWindowRef.current.close();
      return;
    }

    const el = sceneRef.current;
    if (!el) return;

    // Guard against double-clicks while requestWindow is in-flight.
    setPending(true);
    let pipWindow: Window;
    try {
      pipWindow = await window.documentPictureInPicture!.requestWindow({ width, height });
    } finally {
      setPending(false);
    }
    pipWindowRef.current = pipWindow;

    // Capture the element's DOM position *after* the await so the references
    // reflect the actual DOM state at the moment we re-parent — not a
    // potentially stale snapshot from before the async gap.
    const parent = el.parentElement;
    const nextSibling = el.nextSibling;

    // Clone all same-origin stylesheets so Tailwind and tokens work.
    // This captures stylesheets that exist at the time the PiP window opens.
    // Dynamically injected <link> elements added *after* this point (e.g. lazy-
    // loaded CSS chunks) won't appear in the PiP window — acceptable for this
    // use case since the dojo page is fully loaded before pop-out is triggered.
    for (const sheet of document.styleSheets) {
      try {
        const style = document.createElement("style");
        style.textContent = [...sheet.cssRules].map((r) => r.cssText).join("\n");
        pipWindow.document.head.appendChild(style);
      } catch {
        // Cross-origin sheet — skip silently.
      }
    }

    // Carry over the theme class (e.g. "dark") so design tokens match.
    pipWindow.document.documentElement.className = document.documentElement.className;

    // Keep the PiP window's theme in sync if the user toggles dark/light mode
    // in the main window while the PiP window is open.
    const themeObserver = new MutationObserver(() => {
      pipWindow.document.documentElement.className = document.documentElement.className;
    });
    themeObserver.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["class"],
    });

    // Make the body a flex column so the sceneRef div's flex-1 fills the
    // full PiP window height and the layout adapts when the window is resized.
    pipWindow.document.body.style.cssText = "margin:0;overflow:hidden;height:100vh;display:flex;flex-direction:column";

    // Move the scene subtree into the PiP window.
    pipWindow.document.body.appendChild(el);
    setIsOpen(true);

    // Restore the subtree to its original position when the window closes.
    // { once: true } ensures the listener is auto-removed after firing.
    pipWindow.addEventListener("pagehide", () => {
      themeObserver.disconnect();
      pipWindowRef.current = null;
      setIsOpen(false);
      if (parent) {
        // Guard against the sibling no longer being a child of parent (e.g.
        // due to React re-renders while the PiP window was open), which would
        // cause insertBefore to throw a DOMException.
        if (nextSibling && nextSibling.parentNode === parent) {
          parent.insertBefore(el, nextSibling);
        } else {
          parent.appendChild(el);
        }
      }
    }, { once: true });
  }, [isSupported, pending, width, height]);

  return { isSupported, isOpen, toggle, sceneRef } as const;
}
