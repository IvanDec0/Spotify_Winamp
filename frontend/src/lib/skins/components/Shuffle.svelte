<script lang="ts">
  import { t } from "../../../shared/i18n";
  import Icon from "../../components/Icon.svelte";
  import { player } from "../../player.svelte";

  interface Props {
    showLabel?: boolean;
  }

  let { showLabel = false }: Props = $props();
</script>

<button
  class="cad-shuffle-button"
  class:is-active={player.state.shuffle}
  type="button"
  disabled={!player.state.canControl}
  aria-label={player.state.shuffle ? t("transport.shuffleOff") : t("transport.shuffleOn")}
  aria-pressed={player.state.shuffle}
  title={t("transport.shuffleShortcut")}
  data-component="shuffle"
  onclick={() => void player.toggleShuffle()}
>
  <Icon name="shuffle" size={12} />
  {#if showLabel}<span>{t("transport.shuffle")}</span>{/if}
</button>
