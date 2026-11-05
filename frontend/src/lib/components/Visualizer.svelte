<script lang="ts">
  import { onMount } from "svelte";
  import type { VisualizerFrame } from "../../shared/api/types";
  import type { VisualizerMode } from "../preferences.svelte";
  import { visualizerData } from "../visualizer-data.svelte";
  import {
    bandValue,
    beatEnvelope,
    clamp,
    gridPoint,
    seededUnit,
    signalEnergy,
    spectrumBar,
    trackSeed,
    waveSample,
    waveformGain,
    waveformValue,
  } from "../visualizer";

  interface Props {
    mode: VisualizerMode;
    trackId: string;
    playing: boolean;
    reducedMotion: boolean;
    accent: string;
    secondary: string;
  }

  let { mode, trackId, playing, reducedMotion, accent, secondary }: Props = $props();

  let canvas: HTMLCanvasElement | null = $state(null);
  let context: CanvasRenderingContext2D | null = null;
  let width = 0;
  let height = 0;
  let raf = 0;
  let running = false;
  let ready = $state(false);
  let visible = $state(true);
  let lastFrameAt = 0;

  const frameInterval = 1000 / 30;

  function colorWithAlpha(hex: string, alpha: number): string {
    const value = hex.replace("#", "");
    if (value.length !== 6) return `rgba(255, 255, 255, ${alpha})`;
    const number = Number.parseInt(value, 16);
    const red = (number >> 16) & 255;
    const green = (number >> 8) & 255;
    const blue = number & 255;
    return `rgba(${red}, ${green}, ${blue}, ${alpha})`;
  }

  function resizeCanvas(): void {
    if (!canvas || !context) return;
    const rect = canvas.getBoundingClientRect();
    const ratio = Math.min(globalThis.devicePixelRatio || 1, 2);
    width = Math.max(0, rect.width);
    height = Math.max(0, rect.height);
    canvas.width = Math.round(width * ratio);
    canvas.height = Math.round(height * ratio);
    context.setTransform(ratio, 0, 0, ratio, 0, 0);
  }

  function drawSpectrum(
    time: number,
    seed: number,
    isPlaying: boolean,
    live: VisualizerFrame | null,
  ): void {
    if (!context) return;
    const count = Math.round(clamp(Math.floor(width / 9), 20, 48));
    const step = (width - 24) / count;
    const baseline = height - 7;
    const barWidth = Math.max(1.2, step * 0.32);

    for (let index = 0; index < count; index += 1) {
      const value = live
        ? bandValue(live.bands, index, count)
        : spectrumBar(index, count, time, seed, isPlaying);
      const barHeight = 2 + value * 28;
      const x = 12 + index * step;
      context.fillStyle = colorWithAlpha(accent, 0.035 + value * 0.16);
      context.fillRect(x, baseline - barHeight, barWidth, barHeight);
      context.fillStyle = colorWithAlpha(secondary, 0.025 + value * 0.07);
      context.fillRect(x, baseline + 1.5, barWidth, barHeight * 0.35);
    }
  }

  function drawRadial(
    time: number,
    seed: number,
    isPlaying: boolean,
    live: VisualizerFrame | null,
  ): void {
    if (!context) return;
    const centreX = width * 0.5;
    const centreY = height * 0.8;
    const tempo = 78 + seed * 44;
    const energy = live ? clamp(live.rms, 0, 1) : signalEnergy(time, seed, isPlaying);
    const beat = live ? energy : isPlaying ? beatEnvelope(time, tempo) : 0;
    const base = 22 + beat * 4;

    const glow = context.createRadialGradient(centreX, centreY, 0, centreX, centreY, 58);
    glow.addColorStop(0, colorWithAlpha(accent, 0.045 + energy * 0.04));
    glow.addColorStop(1, colorWithAlpha(accent, 0));
    context.fillStyle = glow;
    context.fillRect(centreX - 60, centreY - 60, 120, 120);

    for (let ring = 0; ring < 4; ring += 1) {
      const radius = base + ring * 9 + energy * 2.5;
      context.beginPath();
      context.arc(centreX, centreY, radius, 0, Math.PI * 2);
      context.strokeStyle = colorWithAlpha(accent, 0.16 - ring * 0.028);
      context.lineWidth = 1;
      context.stroke();
    }

    context.beginPath();
    context.arc(centreX, centreY, base + 30, time * 0.55, time * 0.55 + Math.PI * 0.72);
    context.strokeStyle = colorWithAlpha(secondary, 0.18);
    context.lineWidth = 1.4;
    context.lineCap = "round";
    context.stroke();
  }

  function drawOscilloscope(
    time: number,
    seed: number,
    isPlaying: boolean,
    live: VisualizerFrame | null,
  ): void {
    if (!context) return;
    const centreY = height - 52;
    const energy = live ? clamp(live.rms, 0, 1) : signalEnergy(time, seed, isPlaying);
    const gain = live ? waveformGain(live.waveform) : 1;
    const amplitude = (live ? 6 : 4) + energy * (live ? 18 : 13);
    const points = 84;
    const startX = 16;
    const span = width - 32;

    context.beginPath();
    for (let index = 0; index <= points; index += 1) {
      const progress = index / points;
      const sample = live
        ? waveformValue(live.waveform, progress)
        : waveSample(progress, time, seed, isPlaying);
      const x = startX + progress * span;
      const y = centreY + sample * gain * amplitude;
      if (index === 0) context.moveTo(x, y);
      else context.lineTo(x, y);
    }
    context.strokeStyle = colorWithAlpha(accent, 0.055);
    context.lineWidth = 5;
    context.lineJoin = "round";
    context.stroke();

    context.strokeStyle = colorWithAlpha(accent, 0.28);
    context.lineWidth = 1.05;
    context.stroke();

    context.beginPath();
    context.moveTo(startX, centreY);
    context.lineTo(startX + span, centreY);
    context.strokeStyle = colorWithAlpha(secondary, 0.06);
    context.lineWidth = 0.8;
    context.stroke();
  }

  function drawGrid(
    time: number,
    seed: number,
    isPlaying: boolean,
    live: VisualizerFrame | null,
  ): void {
    if (!context) return;
    const columns = Math.max(10, Math.floor((width - 24) / 13));
    const rows = Math.max(6, Math.floor((height - 24) / 13));
    const drift = isPlaying ? (time * 2.6) % 13 : 0;
    const energy = live ? clamp(live.rms, 0, 1) : signalEnergy(time, seed, isPlaying);

    for (let column = 0; column <= columns; column += 1) {
      for (let row = 0; row <= rows; row += 1) {
        const x = 12 + column * 13 + drift;
        const y = 12 + row * 13;
        if (x > width - 8) continue;
        const index = row * (columns + 1) + column;
        if (seededUnit(seed * 811, index) < 0.2) continue;
        const value = live
          ? clamp(
              bandValue(live.bands, column, columns + 1) *
                (0.35 + (1 - Math.abs(row / rows - 0.72)) * 0.65) *
                (0.6 + energy * 0.8),
              0,
              1,
            )
          : gridPoint(
              (x - 12) / Math.max(1, width - 24),
              (y - 12) / Math.max(1, height - 24),
              time,
              seed,
              isPlaying,
            );
        context.beginPath();
        context.arc(x, y, 0.55 + value * 1.15, 0, Math.PI * 2);
        context.fillStyle =
          value > 0.78
            ? colorWithAlpha(secondary, 0.07 + value * 0.24)
            : colorWithAlpha(accent, 0.035 + value * 0.18);
        context.fill();
      }
    }
  }

  function renderAt(time: number): void {
    if (!context || width === 0 || height === 0) return;
    const seed = trackSeed(trackId);
    const motionTime = reducedMotion ? 0 : time;
    const motionPlaying = reducedMotion ? false : playing;
    const live = !reducedMotion && visualizerData.isFresh() ? visualizerData.frame : null;
    context.clearRect(0, 0, width, height);

    switch (mode) {
      case "radial":
        drawRadial(motionTime, seed, motionPlaying, live);
        break;
      case "oscilloscope":
        drawOscilloscope(motionTime, seed, motionPlaying, live);
        break;
      case "grid":
        drawGrid(motionTime, seed, motionPlaying, live);
        break;
      default:
        drawSpectrum(motionTime, seed, motionPlaying, live);
    }
  }

  function frame(now: number): void {
    if (!running) return;
    if (now - lastFrameAt >= frameInterval) {
      lastFrameAt = now;
      renderAt(now / 1000);
    }
    raf = requestAnimationFrame(frame);
  }

  function startLoop(): void {
    if (running || !context) return;
    running = true;
    lastFrameAt = 0;
    raf = requestAnimationFrame(frame);
  }

  function stopLoop(): void {
    running = false;
    cancelAnimationFrame(raf);
    raf = 0;
  }

  $effect(() => {
    // Re-read palette and drawing mode whenever they change.
    void mode;
    void trackId;
    void accent;
    void secondary;
    const animate = playing && !reducedMotion && visible;
    visualizerData.setActive(animate);
    if (!ready) return;
    if (!animate) {
      stopLoop();
      renderAt(reducedMotion ? 0 : performance.now() / 1000);
      return;
    }
    stopLoop();
    startLoop();
  });

  onMount(() => {
    if (!canvas) return;
    context = canvas.getContext("2d");
    resizeCanvas();
    renderAt(performance.now() / 1000);
    ready = true;

    const updateVisibility = (): void => {
      visible = !document.hidden;
    };
    updateVisibility();
    document.addEventListener("visibilitychange", updateVisibility);

    let observer: ResizeObserver | null = null;
    if (typeof ResizeObserver !== "undefined") {
      observer = new ResizeObserver(() => {
        resizeCanvas();
        renderAt(performance.now() / 1000);
      });
      observer.observe(canvas);
    }

    return () => {
      stopLoop();
      visualizerData.setActive(false);
      ready = false;
      document.removeEventListener("visibilitychange", updateVisibility);
      observer?.disconnect();
      context = null;
    };
  });
</script>

<canvas bind:this={canvas} class="visualizer" aria-hidden="true"></canvas>

<style>
  .visualizer {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    opacity: 0.9;
    pointer-events: none;
    mix-blend-mode: screen;
  }
</style>
