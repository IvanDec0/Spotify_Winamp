<script lang="ts">
  import { player } from "../../player.svelte";

  interface Props {
    size?: number;
    frame?: boolean;
    reflection?: boolean;
  }

  let { size = 88, frame = false, reflection = false }: Props = $props();

  const variants = ["meridian", "tides", "orbit", "afterimage", "bloom", "nightbus"] as const;

  let variant = $derived(
    variants.includes(player.state.track.cover as (typeof variants)[number])
      ? player.state.track.cover
      : "meridian",
  );
</script>

<div
  class={`cad-cover cad-cover--${variant}`}
  class:is-framed={frame}
  class:is-reflected={reflection}
  style={`--cover-size: ${size}px`}
  role="img"
  aria-label={`Album art for ${player.state.track.title}`}
  data-component="album-art"
>
  <div class="cad-cover-inner">
    <svg viewBox="0 0 100 100" fill="none" aria-hidden="true">
      <circle cx="50" cy="50" r="26" stroke="currentColor" stroke-width="1.1" opacity=".62" />
      <circle cx="50" cy="50" r="16" stroke="currentColor" stroke-width=".7" opacity=".28" />
      <path d="M50 10v80" stroke="currentColor" stroke-width="1.2" opacity=".9" />
      <path d="M14 50h72" stroke="currentColor" stroke-width=".6" opacity=".28" />
      <circle cx="50" cy="50" r="2.7" fill="currentColor" stroke="none" />
    </svg>
  </div>
</div>
