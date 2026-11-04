<script lang="ts">
  import { RepeatMode, Status } from "../../../shared/api/types";
  import Icon from "../../components/Icon.svelte";
  import { player } from "../../player.svelte";

  interface Props {
    size?: "compact" | "regular" | "large";
    showShuffle?: boolean;
    showRepeat?: boolean;
  }

  let { size = "regular", showShuffle = true, showRepeat = true }: Props = $props();

  let playing = $derived(player.state.status === Status.StatusPlaying);
  let repeatLabel = $derived(
    player.state.repeat === RepeatMode.RepeatTrack
      ? "Repeat one track"
      : player.state.repeat === RepeatMode.RepeatContext
        ? "Repeat queue"
        : "Repeat off",
  );
</script>

<div
  class="cad-transport"
  class:is-compact={size === "compact"}
  class:is-large={size === "large"}
  aria-label="Playback controls"
  data-component="transport-controls"
>
  {#if showShuffle}
    <button
      class="cad-control"
      class:is-active={player.state.shuffle}
      type="button"
      disabled={!player.state.canControl}
      aria-label={player.state.shuffle ? "Turn shuffle off" : "Turn shuffle on"}
      aria-pressed={player.state.shuffle}
      title="Shuffle (S)"
      onclick={() => void player.toggleShuffle()}
    >
      <Icon name="shuffle" size={13} />
    </button>
  {/if}

  <button
    class="cad-control"
    type="button"
    disabled={!player.state.canControl}
    aria-label="Previous track"
    title="Previous (Shift+←)"
    onclick={() => void player.previous()}
  >
    <Icon name="previous" size={14} />
  </button>

  <button
    class="cad-play"
    class:is-playing={playing}
    type="button"
    disabled={!player.state.canControl}
    aria-label={playing ? "Pause" : "Play"}
    title={playing ? "Pause (Space)" : "Play (Space)"}
    onclick={() => void player.toggle()}
  >
    <Icon name={playing ? "pause" : "play"} size={14} />
  </button>

  <button
    class="cad-control"
    type="button"
    disabled={!player.state.canControl}
    aria-label="Next track"
    title="Next (Shift+→)"
    onclick={() => void player.next()}
  >
    <Icon name="next" size={14} />
  </button>

  {#if showRepeat}
    <button
      class="cad-control"
      class:is-active={player.state.repeat !== RepeatMode.RepeatOff}
      type="button"
      disabled={!player.state.canControl}
      aria-label={repeatLabel}
      aria-pressed={player.state.repeat !== RepeatMode.RepeatOff}
      title={`${repeatLabel} (R)`}
      onclick={() => void player.cycleRepeat()}
    >
      <Icon name="repeat" size={13} />
      {#if player.state.repeat === RepeatMode.RepeatTrack}
        <span class="cad-repeat-one" aria-hidden="true">1</span>
      {/if}
    </button>
  {/if}
</div>
