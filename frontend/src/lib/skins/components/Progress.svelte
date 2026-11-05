<script lang="ts">
  import { onMount } from "svelte";
  import { Status } from "../../../shared/api/types";
  import { t } from "../../../shared/i18n";
  import { formatTime } from "../../format";
  import { player } from "../../player.svelte";
  import { clamp } from "../../visualizer";

  interface Props {
    showTime?: boolean;
    showRemaining?: boolean;
  }

  let { showTime = true, showRemaining = false }: Props = $props();

  let dragging = $state(false);
  let dragValue = $state(0);
  let visible = $state(true);
  let interpolated = $state(player.state.progressMs);

  let playing = $derived(player.state.status === Status.StatusPlaying);
  let duration = $derived(Math.max(1, player.state.durationMs));
  let shown = $derived(dragging ? dragValue : clamp(interpolated, 0, player.state.durationMs));
  let percent = $derived(clamp((shown / duration) * 100, 0, 100));

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
</script>

<div class="cad-progress" data-component="progress">
  <input
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
    style={`--progress: ${percent}%`}
    onpointerdown={beginDrag}
    oninput={update}
    onchange={commit}
  />
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
