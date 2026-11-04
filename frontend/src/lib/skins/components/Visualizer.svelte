<script lang="ts">
  import { onMount } from "svelte";
  import { Status } from "../../../shared/api/types";
  import { player } from "../../player.svelte";
  import { preferences } from "../../preferences.svelte";
  import { skins } from "../../skins.svelte";
  import {
    beatEnvelope,
    clamp,
    gridPoint,
    seededUnit,
    signalEnergy,
    spectrumBar,
    trackSeed,
    waveSample,
  } from "../../visualizer";

  interface Props {
    mode?: "spectrum" | "radial" | "oscilloscope" | "grid";
    layer?: "background" | "inline";
    intensity?: number;
  }

  let { mode: declaredMode, layer = "background", intensity = 0.6 }: Props = $props();

  const compact = $derived(layer === "inline");

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

  let modes = $derived(skins.visualizerOptions);
  let mode = $derived(
    modes.includes(skins.visualizer)
      ? skins.visualizer
      : declaredMode && modes.includes(declaredMode)
        ? declaredMode
        : modes[0],
  );
  let playing = $derived(player.state.status === Status.StatusPlaying);
  let reducedMotion = $derived(preferences.reducedMotion);
  let accent = $derived(skins.colors.accent);
  let secondary = $derived(skins.colors.secondary);
  let energyScale = $derived(clamp(intensity, 0, 1));

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

  function drawSpectrum(time: number, seed: number, active: boolean): void {
    if (!context) return;
    const count = Math.round(clamp(Math.floor(width / (compact ? 7 : 9)), compact ? 14 : 20, 48));
    const step = (width - 24) / count;
    const baseline = height - (compact ? 4 : 7);
    const barWidth = Math.max(1.2, step * 0.32);

    for (let index = 0; index < count; index += 1) {
      const value = spectrumBar(index, count, time, seed, active) * energyScale;
      const barHeight = 2 + value * (compact ? 22 : 28);
      const x = 12 + index * step;
      context.globalAlpha = 0.035 + value * 0.16;
      context.fillStyle = accent;
      context.fillRect(x, baseline - barHeight, barWidth, barHeight);
      context.globalAlpha = 0.025 + value * 0.07;
      context.fillStyle = secondary;
      context.fillRect(x, baseline + 1.5, barWidth, barHeight * 0.35);
    }
    context.globalAlpha = 1;
  }

  function drawRadial(time: number, seed: number, active: boolean): void {
    if (!context) return;
    const centreX = width * 0.5;
    const centreY = height * (compact ? 0.56 : 0.8);
    const tempo = 78 + seed * 44;
    const beat = active ? beatEnvelope(time, tempo) : 0;
    const energy = signalEnergy(time, seed, active) * energyScale;
    const base = (compact ? 12 : 22) + beat * (compact ? 2.5 : 4);
    const glowRadius = compact ? 34 : 58;

    const glow = context.createRadialGradient(centreX, centreY, 0, centreX, centreY, glowRadius);
    glow.addColorStop(0, accent);
    glow.addColorStop(1, "transparent");
    context.globalAlpha = 0.05 + energy * 0.05;
    context.fillStyle = glow;
    context.fillRect(centreX - glowRadius, centreY - glowRadius, glowRadius * 2, glowRadius * 2);

    for (let ring = 0; ring < 4; ring += 1) {
      const radius = base + ring * (compact ? 5 : 9) + energy * 2.5;
      context.beginPath();
      context.arc(centreX, centreY, radius, 0, Math.PI * 2);
      context.globalAlpha = 0.16 - ring * 0.028;
      context.strokeStyle = accent;
      context.lineWidth = 1;
      context.stroke();
    }

    context.beginPath();
    context.arc(
      centreX,
      centreY,
      base + (compact ? 16 : 30),
      time * 0.55,
      time * 0.55 + Math.PI * 0.72,
    );
    context.globalAlpha = 0.22;
    context.strokeStyle = secondary;
    context.lineWidth = 1.4;
    context.lineCap = "round";
    context.stroke();
    context.globalAlpha = 1;
  }

  function drawOscilloscope(time: number, seed: number, active: boolean): void {
    if (!context) return;
    const centreY = compact ? height * 0.55 : height - 52;
    const energy = signalEnergy(time, seed, active) * energyScale;
    const amplitude = 4 + energy * (compact ? 8 : 13);
    const points = 84;
    const startX = 16;
    const span = width - 32;

    context.beginPath();
    for (let index = 0; index <= points; index += 1) {
      const progress = index / points;
      const sample = waveSample(progress, time, seed, active);
      const x = startX + progress * span;
      const y = centreY + sample * amplitude;
      if (index === 0) context.moveTo(x, y);
      else context.lineTo(x, y);
    }
    context.globalAlpha = 0.07;
    context.strokeStyle = accent;
    context.lineWidth = 5;
    context.lineJoin = "round";
    context.stroke();

    context.globalAlpha = 0.28;
    context.lineWidth = 1.05;
    context.stroke();

    context.beginPath();
    context.moveTo(startX, centreY);
    context.lineTo(startX + span, centreY);
    context.globalAlpha = 0.06;
    context.strokeStyle = secondary;
    context.lineWidth = 0.8;
    context.stroke();
    context.globalAlpha = 1;
  }

  function drawGrid(time: number, seed: number, active: boolean): void {
    if (!context) return;
    const columns = Math.max(10, Math.floor((width - 24) / 13));
    const rows = Math.max(6, Math.floor((height - 24) / 13));
    const drift = active ? (time * 2.6) % 13 : 0;

    for (let column = 0; column <= columns; column += 1) {
      for (let row = 0; row <= rows; row += 1) {
        const x = 12 + column * 13 + drift;
        const y = 12 + row * 13;
        if (x > width - 8) continue;
        const index = row * (columns + 1) + column;
        if (seededUnit(seed * 811, index) < 0.2) continue;
        const value =
          gridPoint(
            (x - 12) / Math.max(1, width - 24),
            (y - 12) / Math.max(1, height - 24),
            time,
            seed,
            active,
          ) * energyScale;
        context.beginPath();
        context.arc(x, y, 0.55 + value * 1.15, 0, Math.PI * 2);
        context.globalAlpha = value > 0.78 ? 0.07 + value * 0.24 : 0.035 + value * 0.18;
        context.fillStyle = value > 0.78 ? secondary : accent;
        context.fill();
      }
    }
    context.globalAlpha = 1;
  }

  function renderAt(time: number): void {
    if (!context || width === 0 || height === 0) return;
    const seed = trackSeed(player.state.track.id);
    const motionTime = reducedMotion ? 0 : time;
    const motionPlaying = reducedMotion ? false : playing;
    context.clearRect(0, 0, width, height);

    switch (mode) {
      case "radial":
        drawRadial(motionTime, seed, motionPlaying);
        break;
      case "oscilloscope":
        drawOscilloscope(motionTime, seed, motionPlaying);
        break;
      case "grid":
        drawGrid(motionTime, seed, motionPlaying);
        break;
      default:
        drawSpectrum(motionTime, seed, motionPlaying);
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
    void mode;
    void accent;
    void secondary;
    void energyScale;
    void player.state.track.id;
    const animate = playing && !reducedMotion && visible;
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
      ready = false;
      document.removeEventListener("visibilitychange", updateVisibility);
      observer?.disconnect();
      context = null;
    };
  });
</script>

<canvas
  bind:this={canvas}
  class="cad-visualizer"
  class:is-inline={layer === "inline"}
  class:is-background={layer === "background"}
  aria-hidden="true"
></canvas>
