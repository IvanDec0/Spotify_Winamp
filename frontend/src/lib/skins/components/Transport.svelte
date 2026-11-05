<script lang="ts">
  import { RepeatMode, Status } from "../../../shared/api/types";
  import { t } from "../../../shared/i18n";
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
      ? t("transport.repeatTrack")
      : player.state.repeat === RepeatMode.RepeatContext
        ? t("transport.repeatContext")
        : t("transport.repeatOff"),
  );
</script>

<div
  class="cad-transport"
  class:is-compact={size === "compact"}
  class:is-large={size === "large"}
  aria-label={t("transport.controls")}
  data-component="transport-controls"
>
  {#if showShuffle}
    <button
      class="cad-control"
      class:is-active={player.state.shuffle}
      type="button"
      disabled={!player.state.canControl}
      aria-label={player.state.shuffle ? t("transport.shuffleOff") : t("transport.shuffleOn")}
      aria-pressed={player.state.shuffle}
      title={t("transport.shuffleShortcut")}
      onclick={() => void player.toggleShuffle()}
    >
      <Icon name="shuffle" size={13} />
    </button>
  {/if}

  <button
    class="cad-control"
    type="button"
    disabled={!player.state.canControl}
    aria-label={t("transport.previousTrack")}
    title={t("transport.previousShortcut")}
    onclick={() => void player.previous()}
  >
    <Icon name="previous" size={14} />
  </button>

  <button
    class="cad-play"
    class:is-playing={playing}
    type="button"
    disabled={!player.state.canControl}
    aria-label={playing ? t("transport.pause") : t("transport.play")}
    title={playing ? t("transport.pauseShortcut") : t("transport.playShortcut")}
    onclick={() => void player.toggle()}
  >
    <Icon name={playing ? "pause" : "play"} size={14} />
  </button>

  <button
    class="cad-control"
    type="button"
    disabled={!player.state.canControl}
    aria-label={t("transport.nextTrack")}
    title={t("transport.nextShortcut")}
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
      title={t("transport.repeatShortcut", { label: repeatLabel })}
      onclick={() => void player.cycleRepeat()}
    >
      <Icon name="repeat" size={13} />
      {#if player.state.repeat === RepeatMode.RepeatTrack}
        <span class="cad-repeat-one" aria-hidden="true">1</span>
      {/if}
    </button>
  {/if}
</div>
