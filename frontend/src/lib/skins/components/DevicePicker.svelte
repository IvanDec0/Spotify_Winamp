<script lang="ts">
  import Icon from "../../components/Icon.svelte";
  import { player } from "../../player.svelte";
  import { skins } from "../../skins.svelte";

  interface Props {
    compact?: boolean;
  }

  let { compact = false }: Props = $props();
  let deviceLabel = $derived(player.state.device.name || "No device");
</script>

<button
  class="cad-device-picker"
  class:is-compact={compact}
  type="button"
  title="Choose a playback device"
  aria-label={`Playback device: ${deviceLabel}. Choose a device`}
  data-component="device-picker"
  onclick={() => skins.openDevices()}
>
  <Icon name="device" size={11} />
  <span class="cad-device-name">{deviceLabel}</span>
</button>
