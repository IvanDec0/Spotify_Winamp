<script lang="ts">
  import { t } from "../../../shared/i18n";
  import Icon from "../../components/Icon.svelte";
  import { skins } from "../../skins.svelte";

  interface Props {
    showLabel?: boolean;
  }

  let { showLabel = false }: Props = $props();
</script>

<button
  class="cad-settings-button"
  type="button"
  title={t("browse.settings")}
  aria-label={t("browse.settings")}
  data-component="settings-button"
  onclick={() => skins.openSettings()}
>
  <Icon name="settings" size={13} />
  {#if showLabel}<span>Skins</span>{/if}
</button>
