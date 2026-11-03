<script lang="ts">
  import Icon from "../../components/Icon.svelte";
  import { player } from "../../player.svelte";
  import { skins } from "../../skins.svelte";

  interface Props {
    compact?: boolean;
  }

  let { compact = false }: Props = $props();
</script>

<button
  class="cad-device-picker"
  class:is-compact={compact}
  type="button"
  title="Choose a playback device"
  aria-label={`Playback device: ${player.state.device.name}. Choose a device`}
  data-component="device-picker"
  onclick={() => skins.openDevices()}
>
  <Icon name="device" size={11} />
  <span class="cad-device-name">{player.state.device.name}</span>
</button>
