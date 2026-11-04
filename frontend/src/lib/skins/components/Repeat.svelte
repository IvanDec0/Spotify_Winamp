<script lang="ts">
  import { RepeatMode } from "../../../shared/api/types";
  import Icon from "../../components/Icon.svelte";
  import { player } from "../../player.svelte";

  interface Props {
    showLabel?: boolean;
  }

  let { showLabel = false }: Props = $props();

  let active = $derived(player.state.repeat !== RepeatMode.RepeatOff);
  let label = $derived(
    player.state.repeat === RepeatMode.RepeatTrack
      ? "Repeat one track"
      : player.state.repeat === RepeatMode.RepeatContext
        ? "Repeat queue"
        : "Repeat off",
  );
</script>

<button
  class="cad-repeat-button"
  class:is-active={active}
  type="button"
  disabled={!player.state.canControl}
  aria-label={label}
  aria-pressed={active}
  title={`${label} (R)`}
  data-component="repeat"
  onclick={() => void player.cycleRepeat()}
>
  <Icon name="repeat" size={12} />
  {#if showLabel}<span>Repeat</span>{/if}
  {#if player.state.repeat === RepeatMode.RepeatTrack}
    <span class="cad-repeat-one" aria-hidden="true">1</span>
  {/if}
</button>
