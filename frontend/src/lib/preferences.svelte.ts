export const themes = [
  { id: "graphite", label: "Graphite", accent: "#5ef0ff", secondary: "#7d8cff" },
  { id: "ice", label: "Ice", accent: "#8be9ff", secondary: "#4d9fff" },
  { id: "violet", label: "Violet", accent: "#c08bff", secondary: "#ff7ad9" },
  { id: "amber", label: "Amber", accent: "#ffc76b", secondary: "#ff8a4c" },
  { id: "acid", label: "Acid", accent: "#b8ff5e", secondary: "#5effc8" },
  { id: "crimson", label: "Crimson", accent: "#ff6b8a", secondary: "#ff9f5e" },
] as const;

export type ThemeId = (typeof themes)[number]["id"];

export const visualizerModes = [
  { id: "spectrum", label: "Spectrum", glyph: "▥" },
  { id: "radial", label: "Radial", glyph: "◎" },
  { id: "oscilloscope", label: "Scope", glyph: "∿" },
  { id: "grid", label: "Grid", glyph: "⣿" },
] as const;

export type VisualizerMode = (typeof visualizerModes)[number]["id"];

export type MotionPreference = "system" | "reduced";

const THEME_KEY = "cadence.theme";
const VISUALIZER_KEY = "cadence.visualizer";
const MOTION_KEY = "cadence.motion";

export function isThemeId(value: unknown): value is ThemeId {
  return themes.some((theme) => theme.id === value);
}

export function isVisualizerMode(value: unknown): value is VisualizerMode {
  return visualizerModes.some((mode) => mode.id === value);
}

export function isMotionPreference(value: unknown): value is MotionPreference {
  return value === "system" || value === "reduced";
}

export function nextVisualizerMode(current: VisualizerMode): VisualizerMode {
  const index = visualizerModes.findIndex((mode) => mode.id === current);
  return visualizerModes[(index + 1) % visualizerModes.length].id;
}

function readStorage(key: string): string | null {
  try {
    return globalThis.localStorage?.getItem(key) ?? null;
  } catch {
    return null;
  }
}

function writeStorage(key: string, value: string): void {
  try {
    globalThis.localStorage?.setItem(key, value);
  } catch {
    // Private browsing or a locked-down webview: preferences stay in memory.
  }
}

function storedTheme(): ThemeId {
  const value = readStorage(THEME_KEY);
  return isThemeId(value) ? value : "graphite";
}

function storedVisualizer(): VisualizerMode {
  const value = readStorage(VISUALIZER_KEY);
  return isVisualizerMode(value) ? value : "spectrum";
}

function storedMotion(): MotionPreference {
  const value = readStorage(MOTION_KEY);
  return isMotionPreference(value) ? value : "system";
}

class PreferencesStore {
  theme = $state<ThemeId>(storedTheme());
  visualizer = $state<VisualizerMode>(storedVisualizer());
  motion = $state<MotionPreference>(storedMotion());
  systemReducedMotion = $state(false);

  #query: MediaQueryList | null = null;
  #onSystemMotionChange = (event: MediaQueryListEvent): void => {
    this.systemReducedMotion = event.matches;
  };

  get reducedMotion(): boolean {
    return this.motion === "reduced" || (this.motion === "system" && this.systemReducedMotion);
  }

  syncSystemMotion(): () => void {
    if (typeof globalThis.matchMedia !== "function") {
      return () => {};
    }

    this.#query = globalThis.matchMedia("(prefers-reduced-motion: reduce)");
    this.systemReducedMotion = this.#query.matches;
    this.#query.addEventListener("change", this.#onSystemMotionChange);

    return () => {
      this.#query?.removeEventListener("change", this.#onSystemMotionChange);
      this.#query = null;
    };
  }

  setTheme(theme: ThemeId): void {
    this.theme = theme;
    writeStorage(THEME_KEY, theme);
  }

  setVisualizer(mode: VisualizerMode): void {
    this.visualizer = mode;
    writeStorage(VISUALIZER_KEY, mode);
  }

  cycleVisualizer(): void {
    this.setVisualizer(nextVisualizerMode(this.visualizer));
  }

  setMotion(motion: MotionPreference): void {
    this.motion = motion;
    writeStorage(MOTION_KEY, motion);
  }

  toggleReducedMotion(): void {
    this.setMotion(this.reducedMotion ? "system" : "reduced");
  }
}

export const preferences = new PreferencesStore();
