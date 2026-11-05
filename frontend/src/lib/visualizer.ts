/**
 * Deterministic pseudo-signals for the simulated visualizer.
 *
 * Cadence never receives audio: the visualizer is a designed simulation driven
 * by playback state, elapsed time and a stable per-track seed. Every function
 * here is pure so the animation can be tested frame by frame.
 */

export function clamp(value: number, minimum: number, maximum: number): number {
  if (!Number.isFinite(value)) return minimum;
  return Math.min(maximum, Math.max(minimum, value));
}

export function hashString(value: string): number {
  let hash = 2166136261;
  for (let index = 0; index < value.length; index += 1) {
    hash ^= value.charCodeAt(index);
    hash = Math.imul(hash, 16777619);
  }
  return hash >>> 0;
}

/** Stable unit value in [0, 1) derived from a seed and an integer step. */
export function seededUnit(seed: number, step: number): number {
  if (!Number.isFinite(seed) || !Number.isFinite(step)) return 0;
  const raw = Math.sin(seed * 12.9898 + step * 78.233) * 43758.5453;
  return raw - Math.floor(raw);
}

export function trackSeed(trackId: string): number {
  return (hashString(trackId) % 10_000) / 10_000;
}

export function trackTempo(trackId: string): number {
  return 78 + Math.round(trackSeed(trackId) * 44);
}

/**
 * Exponential beat envelope. The first beat of every four is accented, which
 * gives the simulation a plausible four-four pulse without audio analysis.
 */
export function beatEnvelope(seconds: number, bpm: number): number {
  const time = Number.isFinite(seconds) && seconds > 0 ? seconds : 0;
  const tempo = clamp(Number.isFinite(bpm) ? bpm : 96, 40, 200);
  const beat = (time * tempo) / 60;
  const phase = beat - Math.floor(beat);
  const decay = Math.exp(-phase * 5.6);
  const accentPhase = ((beat % 4) + 4) % 4;
  const accent = accentPhase < 1 ? Math.exp(-accentPhase * 3.4) * 0.32 : 0;
  return clamp(decay + accent, 0, 1);
}

/** Overall simulated energy. Paused playback breathes slowly instead of pulsing. */
export function signalEnergy(seconds: number, seed: number, playing: boolean): number {
  const time = Number.isFinite(seconds) && seconds >= 0 ? seconds : 0;
  const safeSeed = Number.isFinite(seed) ? seed : 0;
  if (!playing) {
    return clamp(0.2 + 0.06 * (0.5 + 0.5 * Math.sin(time * 0.7 + safeSeed * 9)), 0, 1);
  }
  const tempo = 78 + safeSeed * 44;
  return clamp(0.36 + 0.64 * beatEnvelope(time, tempo), 0, 1);
}

/**
 * One mirrored spectrum bar. `index` may run over any count; the result is
 * clamped and independent of rendering resolution.
 */
export function spectrumBar(
  index: number,
  count: number,
  seconds: number,
  seed: number,
  playing: boolean,
): number {
  const bars = Math.max(1, Math.floor(Number.isFinite(count) ? count : 1));
  const position = clamp(index / bars, 0, 1);
  const time = Number.isFinite(seconds) ? Math.max(0, seconds) : 0;
  const safeSeed = Number.isFinite(seed) ? seed : 0;
  const energy = signalEnergy(time, safeSeed, playing);
  const tilt = 0.32 + 0.68 * Math.sin(Math.PI * Math.min(1, position * 1.18));
  const wave = 0.5 + 0.5 * Math.sin(time * 3.2 + index * 0.82 + safeSeed * 21);
  const jitter = seededUnit(safeSeed * 1000, index + Math.floor(time * 6));
  return clamp((0.14 + wave * 0.6 + jitter * 0.26) * tilt * energy, 0, 1);
}

/** A luminous oscilloscope sample in [-1, 1]. */
export function waveSample(
  progress: number,
  seconds: number,
  seed: number,
  playing: boolean,
): number {
  const x = clamp(Number.isFinite(progress) ? progress : 0, 0, 1);
  const time = Number.isFinite(seconds) ? Math.max(0, seconds) : 0;
  const safeSeed = Number.isFinite(seed) ? seed : 0;
  const energy = signalEnergy(time, safeSeed, playing);
  const carrier = Math.sin(x * Math.PI * 7 + time * 2.4 + safeSeed * 13);
  const harmonic = Math.sin(x * Math.PI * 13 - time * 1.7 + safeSeed * 29) * 0.44;
  const grain = (seededUnit(safeSeed * 10_000, Math.floor(x * 96)) - 0.5) * 0.18;
  return clamp((carrier + harmonic + grain) * energy, -1, 1);
}

/** Brightness of a grid dot in [0, 1]. Coordinates are normalised. */
export function gridPoint(
  x: number,
  y: number,
  seconds: number,
  seed: number,
  playing: boolean,
): number {
  const px = clamp(Number.isFinite(x) ? x : 0, 0, 1);
  const py = clamp(Number.isFinite(y) ? y : 0, 0, 1);
  const time = Number.isFinite(seconds) ? Math.max(0, seconds) : 0;
  const safeSeed = Number.isFinite(seed) ? seed : 0;
  const energy = signalEnergy(time, safeSeed, playing);
  const sweep = 0.5 + 0.5 * Math.sin(px * 18 + py * 14 + time * 0.9 + safeSeed * 17);
  const sparkle = seededUnit(safeSeed * 500, Math.floor(px * 24) * 31 + Math.floor(py * 18));
  const distance = Math.hypot(px - 0.68, py - 0.42);
  const focus = clamp(1 - distance * 1.3, 0, 1);
  return clamp((0.14 + sweep * 0.36 + sparkle * 0.14) * (0.55 + focus * energy * 0.65), 0, 1);
}

/**
 * Maps a spectrum bar index onto the real analysis bands. Bars are spread
 * linearly across the log-spaced bands so a 20-bar canvas still shows all 24
 * bands and a 48-bar canvas interpolates between neighbours.
 */
export function bandValue(bands: readonly number[], index: number, count: number): number {
  if (bands.length === 0 || count <= 0) return 0;
  const position = count <= 1 ? 0 : clamp(index / (count - 1), 0, 1) * (bands.length - 1);
  const lower = Math.floor(position);
  const upper = Math.min(bands.length - 1, lower + 1);
  const mix = position - lower;
  const value = bands[lower] * (1 - mix) + bands[upper] * mix;
  return clamp(Number.isFinite(value) ? value : 0, 0, 1);
}

/** Interpolates a real oscilloscope sample in [-1, 1]. */
export function waveformValue(waveform: readonly number[], progress: number): number {
  if (waveform.length === 0) return 0;
  const position = clamp(Number.isFinite(progress) ? progress : 0, 0, 1) * (waveform.length - 1);
  const lower = Math.floor(position);
  const upper = Math.min(waveform.length - 1, lower + 1);
  const mix = position - lower;
  const value = waveform[lower] * (1 - mix) + waveform[upper] * mix;
  return clamp(Number.isFinite(value) ? value : 0, -1, 1);
}

/**
 * Frame-local oscilloscope gain. Music peaks rarely reach full scale after
 * downsampling, so normalizing each frame keeps the trace visible without
 * amplifying near-silence into noise.
 */
export function waveformGain(waveform: readonly number[]): number {
  let peak = 0;
  for (const value of waveform) {
    const magnitude = Math.abs(value);
    if (Number.isFinite(magnitude) && magnitude > peak) peak = magnitude;
  }
  if (peak <= 0.02) return 0;
  return Math.min(1 / peak, 6);
}
