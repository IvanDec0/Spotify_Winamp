export type IconShape =
  | { kind: "path"; d: string; fill?: boolean }
  | {
      kind: "rect";
      x: number;
      y: number;
      width: number;
      height: number;
      rx?: number;
      fill?: boolean;
    }
  | { kind: "circle"; cx: number; cy: number; r: number; fill?: boolean };

/**
 * Original, hand-drawn interface icons. Shapes are rendered by Icon.svelte on a
 * 24×24 grid; filled shapes opt out of the shared stroke styling.
 */
const iconDefinitions = {
  play: [{ kind: "path", d: "M8.2 5.4 19 12 8.2 18.6Z", fill: true }],
  pause: [
    { kind: "rect", x: 7.1, y: 5.3, width: 3.6, height: 13.4, rx: 1.2, fill: true },
    { kind: "rect", x: 13.3, y: 5.3, width: 3.6, height: 13.4, rx: 1.2, fill: true },
  ],
  previous: [
    { kind: "path", d: "M6.6 5.4v13.2" },
    { kind: "path", d: "M18.2 5.7 8.6 12l9.6 6.3Z", fill: true },
  ],
  next: [
    { kind: "path", d: "M17.4 5.4v13.2" },
    { kind: "path", d: "M5.8 5.7 15.4 12l-9.6 6.3Z", fill: true },
  ],
  shuffle: [
    {
      kind: "path",
      d: "M3 7h2.1c1.2 0 2.3.5 3.1 1.5l5.6 7c.8 1 1.9 1.5 3.1 1.5H22",
    },
    { kind: "path", d: "m18.5 14.5 3.5 2.5-3.5 2.5" },
    { kind: "path", d: "M3 17h2.1c1.2 0 2.3-.5 3.1-1.5l1.1-1.4" },
    { kind: "path", d: "M14.2 9.3l.6-.8c.8-1 1.9-1.5 3.1-1.5H22" },
    { kind: "path", d: "m18.5 4.5 3.5 2.5-3.5 2.5" },
  ],
  repeat: [
    { kind: "path", d: "m17 3 3.5 3.5L17 10" },
    { kind: "path", d: "M20.5 6.5H8.2A4.2 4.2 0 0 0 4 10.7v.8" },
    { kind: "path", d: "m7 21-3.5-3.5L7 14" },
    { kind: "path", d: "M3.5 17.5h12.3a4.2 4.2 0 0 0 4.2-4.2v-.8" },
  ],
  queue: [
    { kind: "path", d: "M3.5 6.5h12M3.5 12h12M3.5 17.5h7.5" },
    { kind: "path", d: "M17.3 14.2 21.5 17.5l-4.2 3.3Z", fill: true },
  ],
  settings: [
    { kind: "path", d: "M3.5 7.5h6.2M15.3 7.5h5.2M3.5 16.5h4.2M13.3 16.5h7.2" },
    { kind: "circle", cx: 12.5, cy: 7.5, r: 2.3 },
    { kind: "circle", cx: 10.5, cy: 16.5, r: 2.3 },
  ],
  device: [
    { kind: "path", d: "M3.5 10v4h3.8l5 4V6l-5 4H3.5Z" },
    { kind: "path", d: "M16 9.4a3.7 3.7 0 0 1 0 5.2M18.6 7a7.2 7.2 0 0 1 0 10" },
  ],
  volume: [
    { kind: "path", d: "M4 10v4h3.6l4.8 3.8V6.2L7.6 10H4Z" },
    { kind: "path", d: "M16 9.5a3.5 3.5 0 0 1 0 5M18.5 7.2a7 7 0 0 1 0 9.6" },
  ],
  volumeMute: [
    { kind: "path", d: "M4 10v4h3.6l4.8 3.8V6.2L7.6 10H4Z" },
    { kind: "path", d: "m16.2 9.5 5 5M21.2 9.5l-5 5" },
  ],
  chevronDown: [{ kind: "path", d: "m6.5 9.2 5.5 5.5 5.5-5.5" }],
  chevronUp: [{ kind: "path", d: "m6.5 14.8 5.5-5.5 5.5 5.5" }],
  close: [{ kind: "path", d: "m6.5 6.5 11 11M17.5 6.5l-11 11" }],
  minimise: [{ kind: "path", d: "M6 12h12" }],
  visualizer: [{ kind: "path", d: "M3.5 15v3M7.8 11v7M12 5.5v12.5M16.2 8.8v9.4M20.5 13v5" }],
  palette: [
    {
      kind: "path",
      d: "M12 3.2a8.8 8.8 0 1 0 0 17.6h1.2a2 2 0 0 0 1.6-3.2 2 2 0 0 1 1.6-3.2h1.4a3 3 0 0 0 3-3.3A8.9 8.9 0 0 0 12 3.2Z",
    },
    { kind: "circle", cx: 7.7, cy: 11.2, r: 0.9, fill: true },
    { kind: "circle", cx: 9.8, cy: 7.3, r: 0.9, fill: true },
    { kind: "circle", cx: 14.2, cy: 7, r: 0.9, fill: true },
    { kind: "circle", cx: 7.6, cy: 15.4, r: 0.9, fill: true },
  ],
  motion: [{ kind: "path", d: "M2.8 12c2.1-4.4 4.3-4.4 6.4 0s4.3 4.4 6.4 0 4.3-4.4 5.6-.9" }],
  check: [{ kind: "path", d: "m5 12.4 4.4 4.4L19 6.7" }],
  info: [
    { kind: "circle", cx: 12, cy: 12, r: 8.6 },
    { kind: "path", d: "M12 11v5.2" },
    { kind: "circle", cx: 12, cy: 7.9, r: 0.9, fill: true },
  ],
} satisfies Record<string, readonly IconShape[]>;

export type IconName = keyof typeof iconDefinitions;

export const icons: Record<IconName, readonly IconShape[]> = iconDefinitions;
