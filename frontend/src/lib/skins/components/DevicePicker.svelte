<script lang="ts">
  import { t } from "../../../shared/i18n";
  import Icon from "../../components/Icon.svelte";
  import { player } from "../../player.svelte";
  import { skins } from "../../skins.svelte";

  interface Props {
    compact?: boolean;
  }

  let { compact = false }: Props = $props();
  let deviceLabel = $derived(player.state.device.name || t("device.noDevice"));
</script>

<button
  class="cad-device-picker"
  class:is-compact={compact}
  type="button"
  title={t("device.choose")}
  aria-label={t("device.chooseLabel", { name: deviceLabel })}
  data-component="device-picker"
  onclick={() => skins.openDevices()}
>
  <Icon name="device" size={11} />
  <span class="cad-device-name">{deviceLabel}</span>
</button>
