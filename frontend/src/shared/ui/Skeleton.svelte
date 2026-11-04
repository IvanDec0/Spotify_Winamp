<script lang="ts">
  interface Props {
    rows?: number;
    height?: number;
    "aria-hidden"?: boolean;
  }

  let { rows = 3, height = 12, "aria-hidden": ariaHidden = true }: Props = $props();

  let count = $derived(Math.max(0, Math.floor(rows)));
</script>

<div class="ui-skeleton" aria-hidden={ariaHidden ? "true" : undefined}>
  {#each Array.from({ length: count }, (_, index) => index) as index (index)}
    <span class="ui-skeleton-row" style:height={`${height}px`}></span>
  {/each}
</div>

<style>
  .ui-skeleton {
    display: grid;
    gap: 6px;
    width: 100%;
  }

  .ui-skeleton-row {
    display: block;
    width: 100%;
    border-radius: 4px;
    background: linear-gradient(
      90deg,
      var(--surface-2, #161b24) 0%,
      var(--surface-3, #1d232e) 50%,
      var(--surface-2, #161b24) 100%
    );
    background-size: 200% 100%;
    animation: ui-skeleton-shimmer 1.4s ease-in-out infinite;
  }

  @keyframes ui-skeleton-shimmer {
    from {
      background-position: 200% 0;
    }
    to {
      background-position: -200% 0;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .ui-skeleton-row {
      animation: none;
    }
  }
</style>
