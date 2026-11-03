<script lang="ts">
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
  aria-label={player.state.shuffle ? "Turn shuffle off" : "Turn shuffle on"}
  aria-pressed={player.state.shuffle}
  title="Shuffle (S)"
  data-component="shuffle"
  onclick={() => void player.toggleShuffle()}
>
  <Icon name="shuffle" size={12} />
  {#if showLabel}<span>Shuffle</span>{/if}
</button>
