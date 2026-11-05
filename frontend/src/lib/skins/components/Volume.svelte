<script lang="ts">
  import { t } from "../../../shared/i18n";
  import Icon from "../../components/Icon.svelte";
  import { player } from "../../player.svelte";
  import { clamp } from "../../visualizer";

  interface Props {
    orientation?: "horizontal" | "vertical";
    showIcon?: boolean;
  }

  let { orientation = "horizontal", showIcon = true }: Props = $props();

  let local = $state(0);
  let previous = 65;

  $effect(() => {
    const next = clamp(player.state.volume, 0, 100);
    local = next;
    if (next > 0) previous = next;
  });

  function update(value: number): void {
    local = clamp(value, 0, 100);
    if (local > 0) previous = local;
    void player.setVolume(Math.round(local));
  }

  function toggleMute(): void {
    if (!player.state.canControl) return;
    update(local > 0 ? 0 : previous || 65);
  }
</script>

<div
  class="cad-volume"
  class:is-vertical={orientation === "vertical"}
  aria-label={t("volume.label")}
  data-component="volume"
>
  {#if showIcon}
    <button
      class="cad-volume-button"
      type="button"
      disabled={!player.state.canControl}
      aria-label={local > 0 ? t("volume.mute") : t("volume.unmute")}
      title={local > 0 ? t("volume.muteShortcut") : t("volume.unmuteShortcut")}
      onclick={toggleMute}
    >
      <Icon name={local > 0 ? "volume" : "volumeMute"} size={12} />
    </button>
  {/if}
  <input
    type="range"
    min="0"
    max="100"
    step="1"
    value={local}
    disabled={!player.state.canControl}
    aria-label={t("volume.level")}
    aria-valuetext={t("volume.percent", { value: Math.round(local) })}
    style={`--progress: ${local}%`}
    oninput={(event) => update((event.currentTarget as HTMLInputElement).valueAsNumber)}
  />
</div>
