<script lang="ts">
  import { RepeatMode } from "../../../shared/api/types";
  import { t } from "../../../shared/i18n";
  import Icon from "../../components/Icon.svelte";
  import { player } from "../../player.svelte";

  interface Props {
    showLabel?: boolean;
  }

  let { showLabel = false }: Props = $props();

  let active = $derived(player.state.repeat !== RepeatMode.RepeatOff);
  let label = $derived(
    player.state.repeat === RepeatMode.RepeatTrack
      ? t("transport.repeatTrack")
      : player.state.repeat === RepeatMode.RepeatContext
        ? t("transport.repeatContext")
        : t("transport.repeatOff"),
  );
</script>

<button
  class="cad-repeat-button"
  class:is-active={active}
  type="button"
  disabled={!player.state.canControl}
  aria-label={label}
  aria-pressed={active}
  title={t("transport.repeatShortcut", { label })}
  data-component="repeat"
  onclick={() => void player.cycleRepeat()}
>
  <Icon name="repeat" size={12} />
  {#if showLabel}<span>{t("transport.repeat")}</span>{/if}
  {#if player.state.repeat === RepeatMode.RepeatTrack}
    <span class="cad-repeat-one" aria-hidden="true">1</span>
  {/if}
</button>
