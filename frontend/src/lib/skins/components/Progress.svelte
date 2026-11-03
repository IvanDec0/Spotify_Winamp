<script lang="ts">
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

  let duration = $derived(Math.max(1, player.state.durationMs));
  let shown = $derived(
    dragging ? dragValue : clamp(player.state.progressMs, 0, player.state.durationMs),
  );
  let percent = $derived(clamp((shown / duration) * 100, 0, 100));

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
    aria-label="Playback position"
    aria-valuetext={`${formatTime(shown / 1000)} of ${formatTime(duration / 1000)}`}
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
