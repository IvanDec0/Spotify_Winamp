<script lang="ts">
  import { onMount } from "svelte";
  import { Status } from "../../shared/api/types";
  import { player } from "../../lib/player.svelte";
  import { preferences } from "../../lib/preferences.svelte";
  import { visualizerData } from "../../lib/visualizer-data.svelte";
  import { bandValue, spectrumBar, trackSeed } from "../../lib/visualizer";

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

  let playing = $derived(player.state.status === Status.StatusPlaying);
  let reducedMotion = $derived(preferences.reducedMotion);

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

  function palette(): { accent: string; secondary: string } {
    const fallback = { accent: "#5ef0ff", secondary: "#7d8cff" };
    if (!canvas || typeof getComputedStyle !== "function") return fallback;
    const style = getComputedStyle(canvas);
    return {
      accent: style.getPropertyValue("--accent").trim() || fallback.accent,
      secondary: style.getPropertyValue("--accent-2").trim() || fallback.secondary,
    };
  }

  function render(time: number): void {
    if (!context || width === 0 || height === 0) return;
    context.clearRect(0, 0, width, height);

    const live = visualizerData.isFresh() ? visualizerData.frame : null;
    const active = playing && !reducedMotion;
    const seed = trackSeed(player.state.track.id);
    const count = Math.max(8, Math.min(24, Math.round(width / 4)));
    const step = width / count;
    const barWidth = Math.max(1, step * 0.55);
    const colors = palette();

    for (let index = 0; index < count; index += 1) {
      const value = live
        ? bandValue(live.bands, index, count)
        : spectrumBar(index, count, time, seed, active) * 0.4;
      const barHeight = 1 + value * (height - 2);
      context.globalAlpha = live ? 0.3 + value * 0.55 : 0.06 + value * 0.16;
      context.fillStyle = colors.accent;
      context.fillRect(index * step, height - barHeight, barWidth, barHeight);
    }

    context.globalAlpha = live ? 0.14 : 0.05;
    context.fillStyle = colors.secondary;
    context.fillRect(0, height - 1, width, 1);
    context.globalAlpha = 1;
  }

  function frame(now: number): void {
    if (!running) return;
    if (now - lastFrameAt >= frameInterval) {
      lastFrameAt = now;
      render(now / 1000);
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
    const animate = playing && !reducedMotion && visible;
    visualizerData.setActive(animate);
    if (!ready) return;
    if (!animate) {
      stopLoop();
      render(performance.now() / 1000);
      return;
    }
    stopLoop();
    startLoop();
  });

  onMount(() => {
    if (!canvas) return;
    context = canvas.getContext("2d");
    resizeCanvas();
    render(performance.now() / 1000);
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
        render(performance.now() / 1000);
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

<canvas bind:this={canvas} class="browse-visualizer" aria-hidden="true"></canvas>

<style>
  .browse-visualizer {
    display: block;
    width: 100%;
    height: 100%;
    opacity: 0.9;
    pointer-events: none;
  }
</style>
