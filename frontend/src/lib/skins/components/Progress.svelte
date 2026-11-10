<script lang="ts">
  import { onMount } from "svelte";
  import { Status } from "../../../shared/api/types";
  import { t } from "../../../shared/i18n";
  import { formatTime } from "../../format";
  import { player } from "../../player.svelte";
  import { clamp, trackSeed } from "../../visualizer";
  import Waveform from "../../components/Waveform.svelte";

  interface Props {
    showTime?: boolean;
    showRemaining?: boolean;
  }

  let { showTime = true, showRemaining = false }: Props = $props();

  let dragging = $state(false);
  let dragValue = $state(0);
  let visible = $state(true);
  let interpolated = $state(player.state.progressMs);
  let hoverPercent = $state<number | null>(null);
  let hoverMs = $state(0);

  let playing = $derived(player.state.status === Status.StatusPlaying);
  let duration = $derived(Math.max(1, player.state.durationMs));
  let shown = $derived(dragging ? dragValue : clamp(interpolated, 0, player.state.durationMs));
  let percent = $derived(clamp((shown / duration) * 100, 0, 100));
  let seed = $derived(trackSeed(player.state.track.id));
  let bubbleVisible = $derived(hoverPercent !== null || dragging);
  let bubblePercent = $derived(dragging ? percent : (hoverPercent ?? 0));
  let bubbleLabel = $derived(formatTime((dragging ? shown : hoverMs) / 1000));

  // The streamed state only reports progress on events, so advance the
  // position locally while playing. The tick stops when paused, dragging or
  // the window is hidden.
  $effect(() => {
    if (!playing || dragging || !visible) {
      interpolated = clamp(player.state.progressMs, 0, player.state.durationMs);
      return;
    }
    interpolated = player.positionMs;
    const timer = setInterval(() => {
      interpolated = player.positionMs;
    }, 250);
    return () => clearInterval(timer);
  });

  onMount(() => {
    const updateVisibility = (): void => {
      visible = !document.hidden;
    };
    updateVisibility();
    document.addEventListener("visibilitychange", updateVisibility);
    return () => document.removeEventListener("visibilitychange", updateVisibility);
  });

  function beginDrag(): void {
    dragging = true;
    dragValue = shown;
  }

  function update(event: Event): void {
    dragValue = clamp((event.currentTarget as HTMLInputElement).valueAsNumber, 0, duration);
  }

  function commit(): void {
    const value = Math.round(clamp(dragValue, 0, duration));
    dragging = false;
    if (player.state.canSeek) {
      void player.seek(value);
    }
  }

  function handleMove(event: PointerEvent): void {
    const target = event.currentTarget as HTMLElement;
    const rect = target.getBoundingClientRect();
    if (rect.width <= 0) return;
    const ratio = clamp((event.clientX - rect.left) / rect.width, 0, 1);
    hoverPercent = ratio * 100;
    hoverMs = ratio * duration;
  }

  function handleLeave(): void {
    if (!dragging) hoverPercent = null;
  }
</script>

<div
  class="cad-progress"
  class:is-playing={playing}
  class:is-dragging={dragging}
  data-component="progress"
  style={`--progress: ${percent}%`}
>
  <div
    class="cad-progress-track"
    role="presentation"
    onpointermove={handleMove}
    onpointerleave={handleLeave}
  >
    <span class="cad-progress-wave" aria-hidden="true">
      <Waveform {percent} {seed} {playing} count={56} />
    </span>
    <span class="cad-progress-playhead" aria-hidden="true"></span>
    {#if bubbleVisible}
      <span
        class="cad-progress-bubble"
        style={`left: clamp(17px, ${bubblePercent}%, calc(100% - 17px))`}
        aria-hidden="true"
      >
        {bubbleLabel}
      </span>
    {/if}
    <input
      class="cad-progress-input"
      type="range"
      min="0"
      max={duration}
      step="100"
      value={shown}
      disabled={!player.state.canSeek}
      aria-label={t("progress.label")}
      aria-valuetext={t("progress.value", {
        position: formatTime(shown / 1000),
        duration: formatTime(duration / 1000),
      })}
      onpointerdown={beginDrag}
      oninput={update}
      onchange={commit}
    />
  </div>
  {#if showTime}
    <div class="cad-progress-time" aria-hidden="true">
      <span>{formatTime(shown / 1000)}</span>
      <span
        >{showRemaining
          ? `-${formatTime((duration - shown) / 1000)}`
          : formatTime(duration / 1000)}</span
      >
    </div>
  {/if}
</div>
