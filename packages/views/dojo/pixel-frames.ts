/**
 * Pixel frame data for Agent Dojo characters.
 *
 * Each frame is a 16×24 string array (one string per row, one char per pixel).
 * Render via PixelCanvas using the PALETTE map below.
 */

/** Maps palette chars to hex colours. null = transparent. */
export const PALETTE: Record<string, string | null> = {
  ".": null,
  H: "#6366f1", h: "#4f46e5",   // body (indigo)
  O: "#fde68a", o: "#d4a853",   // face / skin
  W: "#1e1b4b",                 // eye / sunglasses (dark navy)
  M: "#f472b6",                 // mouth / pink detail
  K: "#374151", k: "#1f2937",   // workstation surface / dark
  S: "#a78bfa", L: "#312e81",   // shirt (light purple) / legs (dark purple)
  B: "#1e1b4b",                 // boots
  A: "#818cf8",                 // arms
  X: "#ef4444",                 // red (defeat × / umbrella stripe)
  G: "#4ade80",                 // green
  Y: "#facc15",                 // yellow (sun / hourglass)
  N: "#d97706",                 // sand / amber
  C: "#7dd3fc",                 // sky blue (umbrella stripe)
};

export type PixelFrame = readonly string[];

// ─── WORKING ────────────────────────────────────────────────────────────────

export const WORKING_A: PixelFrame = [
  "................",
  "................",
  "....HHHHHH......",
  "...HOOOOOOH.....",
  "...HOWOHWOH.....",
  "...HOOOOOOH.....",
  "...HOOMMOH......",
  "....HHHHHH......",
  ".....HHH........",
  "....SSSSSS......",
  "...HSSSSSH......",
  "...HSSHSSH......",
  "..A.SSSS.A......",
  "..A......A......",
  "..AKKKKKKA......",
  "...KKKKKK.......",
  "...KKKKKK.......",
  "....LLLL........",
  "....LLLL........",
  "....L..L........",
  "....L..L........",
  "...BB..BB.......",
  "................",
  "................",
];

export const WORKING_B: PixelFrame = [
  "................",
  "................",
  "....HHHHHH......",
  "...HOOOOOOH.....",
  "...HO..H..H.....",
  "...HOOOOOOH.....",
  "...HOOMMOH......",
  "....HHHHHH......",
  ".....HHH........",
  "....SSSSSS......",
  "...HSSSSSH......",
  "...HSSHSSH......",
  "...A.SSSS.A.....",
  "...A......A.....",
  "...AKKKKKKA.....",
  "...KKKKKK.......",
  "...KKKKKK.......",
  "....LLLL........",
  "....LLLL........",
  "....L..L........",
  "....L..L........",
  "...BB..BB.......",
  "................",
  "................",
];

// ─── WAITING ────────────────────────────────────────────────────────────────

export const WAITING_A: PixelFrame = [
  "................",
  "......YY........",
  "......YY........",
  "....HHHHHH......",
  "...HOOOOOOH.....",
  "...HOWOHWOH.....",
  "...HOOOOOOH.....",
  "...HOO.OOH......",
  "....HHHHHH......",
  ".....HHH........",
  "....SSSSSS......",
  "...HSSSSSH......",
  "...HSSHSSH......",
  "....SSSSSS......",
  "....SSSS........",
  "....LLLL........",
  "....LLLL........",
  "....L..L........",
  "....L..L........",
  "...BB..BB.......",
  "................",
  "................",
  "................",
  "................",
];

export const WAITING_B: PixelFrame = [
  "................",
  "......YY........",
  "......YY........",
  "....HHHHHH......",
  "...HOOOOOOH.....",
  "...HOWOHWOH.....",
  "...HOOOOOOH.....",
  "...HOO.OOH......",
  "....HHHHHH......",
  ".....HHH........",
  "....SSSSSS......",
  "...HSSSSSH......",
  "...HSSHSSH......",
  "....SSSSSS......",
  "....SSSS........",
  "....LLLL........",
  "....LLLL........",
  "....L..L........",
  "....L..L........",
  "....BB..BB......",
  "................",
  "................",
  "................",
  "................",
];

// ─── SLEEPING (online + idle — dozing at workstation) ───────────────────────

export const SLEEPING_A: PixelFrame = [
  "................",
  "................",
  "................",
  "................",
  "................",
  "................",
  "................",
  "................",
  "................",
  "....HHHHHH......",
  "...HOOOOOOH.....",
  "...HO..H..H.....",
  "...HOOOOOOH.....",
  "....HHHHHH......",
  "...HSSSSSH......",
  "...HSSHSSH......",
  "..AASSSSAA......",
  "..KKKKKKKK......",
  "..KKKKKKKK......",
  "....LLLL........",
  "....LLLL........",
  "...BB..BB.......",
  "................",
  "................",
];

export const SLEEPING_B: PixelFrame = [
  "................",
  "................",
  "................",
  "................",
  "................",
  "................",
  "................",
  "................",
  "................",
  "...HHHHHH.......",
  "..HOOOOOOH......",
  "..HO..H..H......",
  "..HOOOOOOH......",
  "...HHHHHH.......",
  "..HSSSSSH.......",
  "..HSSHSSH.......",
  ".AASSSSAA.......",
  ".KKKKKKKK.......",
  ".KKKKKKKK.......",
  "...LLLL.........",
  "...LLLL.........",
  "..BB..BB........",
  "................",
  "................",
];

// ─── VACATIONING (offline / unstable — beach scene) ─────────────────────────

export const VACATIONING_A: PixelFrame = [
  "..............YY",
  ".............YYY",
  "..............YY",
  "................",
  "....X...........",
  "...XCX..........",
  "..XCXCX.........",
  ".XCXCXCX........",
  "XCXCXCXCX.......",
  "....KK..........",
  "....KK..........",
  "....KK..........",
  "................",
  "....HHHHHH......",
  "...HOOOOOOH.....",
  "...HOWWWWOH.....",
  "...HOOOOOOH.....",
  "....HHHHHH......",
  "....SSSSSS......",
  "NNNN.SSSS.NNNNNN",
  "NNNNNNNNNNNNNNNN",
  "NNNNNNNNNNNNNNNN",
  "NNNNNNNNNNNNNNNN",
  "NNNNNNNNNNNNNNNN",
];

export const VACATIONING_B: PixelFrame = [
  "..............YY",
  ".............YYY",
  "..............YY",
  "................",
  ".....X..........",
  "....XCX.........",
  "...XCXCX........",
  "..XCXCXCX.......",
  ".XCXCXCXCX......",
  ".....KK.........",
  ".....KK.........",
  ".....KK.........",
  "................",
  "....HHHHHH......",
  "...HOOOOOOH.....",
  "...HOWWWWOH.....",
  "...HOOOOOOH.....",
  "....HHHHHH......",
  "....SSSSSS......",
  "NNNN.SSSS.NNNNNN",
  "NNNNNNNNNNNNNNNN",
  "NNNNNNNNNNNNNNNN",
  "NNNNNNNNNNNNNNNN",
  "NNNNNNNNNNNNNNNN",
];

// ─── OVERLOADED ─────────────────────────────────────────────────────────────

export const OVERLOADED_A: PixelFrame = [
  "................",
  "..X.........X...",
  "....HHHHHH......",
  "...HOOOOOOH.....",
  "...HOWOHWOH.....",
  "...HOOOOOOH.....",
  "...HOO.OOH......",
  "....HHHHHH......",
  ".....HHH........",
  "....SSSSSS......",
  "..AHSSSSSHA.....",
  "..AHSSHSSHA.....",
  "..A.SSSS.A......",
  "..AKKKKKKA......",
  "..AKKKKKKA......",
  "...KKKKKK.......",
  "...KKKKKK.......",
  "....LLLL........",
  "....LLLL........",
  "....L..L........",
  "...BB..BB.......",
  "................",
  "................",
  "................",
];

export const OVERLOADED_B: PixelFrame = [
  "................",
  "...X.......X....",
  "...HHHHHH.......",
  "..HOOOOOOH......",
  "..HOWOHWOH......",
  "..HOOOOOOH......",
  "..HOO.OOH.......",
  "...HHHHHH.......",
  "....HHH.........",
  "...SSSSSS.......",
  ".AHSSSSSHA......",
  ".AHSSHSSHA......",
  ".A.SSSS.A.......",
  ".AKKKKKKA.......",
  ".AKKKKKKA.......",
  "..KKKKKK........",
  "..KKKKKK........",
  "...LLLL.........",
  "...LLLL.........",
  "...L..L.........",
  "..BB..BB........",
  "................",
  "................",
  "................",
];

// ─── VICTORY ────────────────────────────────────────────────────────────────

export const VICTORY_A: PixelFrame = [
  "................",
  "................",
  "..A........A....",
  "..A........A....",
  "...A.HHHHHHA....",
  "...HOOOOOOH.....",
  "...HOWOHWOH.....",
  "...HOOOOOOH.....",
  "...HOOMMOOH.....",
  "....HHHHHH......",
  ".....HHH........",
  "....SSSSSS......",
  "...HSSSSSH......",
  "...HSSHSSH......",
  "....SSSSSS......",
  "....LLLL........",
  "....LLLL........",
  "................",
  "....L..L........",
  "....L..L........",
  "...BB..BB.......",
  "................",
  "................",
  "................",
];

export const VICTORY_B: PixelFrame = [
  "..A........A....",
  "..A........A....",
  "...A......A.....",
  "....HHHHHH......",
  "...HOOOOOOH.....",
  "...HOWOHWOH.....",
  "...HOOOOOOH.....",
  "...HOOMMOOH.....",
  "....HHHHHH......",
  ".....HHH........",
  "....SSSSSS......",
  "...HSSSSSH......",
  "...HSSHSSH......",
  "....SSSSSS......",
  "....LLLL........",
  "....LLLL........",
  "....L..L........",
  "....L..L........",
  "...BB..BB.......",
  "................",
  "................",
  "................",
  "................",
  "................",
];

// ─── DEFEAT ─────────────────────────────────────────────────────────────────

export const DEFEAT_A: PixelFrame = [
  "................",
  "................",
  "................",
  "................",
  "................",
  "................",
  "................",
  "................",
  "....HHHHHH......",
  "...HOOOOOOH.....",
  "...HO..H..H.....",
  "...HOOOOOOH.....",
  "...HOO.OOH......",
  "....HHHHHH......",
  "...HSSSSSH......",
  "...HSSHSSH......",
  "..AASSSSAA......",
  "..KKKKKKKK......",
  "..KKKKKKKK......",
  "....LLLL........",
  "....LLLL........",
  "...BB..BB.......",
  "................",
  "................",
];

export const DEFEAT_B: PixelFrame = [
  "................",
  "......XX........",
  "......XX........",
  "................",
  "................",
  "................",
  "................",
  "................",
  "...HHHHHH.......",
  "..HOOOOOOH......",
  "..HO..H..H......",
  "..HOOOOOOH......",
  "..HOO.OOH.......",
  "...HHHHHH.......",
  "..HSSSSSH.......",
  "..HSSHSSH.......",
  ".AASSSSAA.......",
  ".KKKKKKKK.......",
  ".KKKKKKKK.......",
  "...LLLL.........",
  "...LLLL.........",
  "..BB..BB........",
  "................",
  "................",
];

// ─── State config ────────────────────────────────────────────────────────────

export type AnimState =
  | "working"
  | "waiting"
  | "sleeping"
  | "vacationing"
  | "overloaded"
  | "victory"
  | "defeat";

interface StateConfig {
  /** Animation frames (cycled in order). */
  frames: readonly PixelFrame[];
  /** Milliseconds between frame advances. */
  intervalMs: number;
}

export const STATE_CONFIG: Record<AnimState, StateConfig> = {
  working:     { frames: [WORKING_A, WORKING_B],         intervalMs: 250  },
  waiting:     { frames: [WAITING_A, WAITING_B],         intervalMs: 600  },
  sleeping:    { frames: [SLEEPING_A, SLEEPING_B],       intervalMs: 1000 },
  vacationing: { frames: [VACATIONING_A, VACATIONING_B], intervalMs: 1400 },
  overloaded:  { frames: [OVERLOADED_A, OVERLOADED_B],   intervalMs: 180  },
  victory:     { frames: [VICTORY_A, VICTORY_B],         intervalMs: 350  },
  defeat:      { frames: [DEFEAT_A, DEFEAT_B],           intervalMs: 700  },
};

/** Duration a transient overlay (VICTORY / DEFEAT) stays visible. */
export const OVERLAY_DURATION_MS = 1500;

export const FRAME_ROWS = 24;
export const FRAME_COLS = 16;
