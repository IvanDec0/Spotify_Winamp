<script lang="ts">
  interface Props {
    rows?: number;
    compact?: boolean;
  }

  let { rows = 7, compact = false }: Props = $props();
</script>

<div class="skeleton-list" aria-hidden="true">
  {#each Array.from({ length: rows }, (_, index) => index) as index (index)}
    <div class="skeleton-row" class:is-compact={compact}>
      <span class="skeleton-art"></span>
      <span class="skeleton-copy">
        <span class="skeleton-line skeleton-line--title"></span>
        <span class="skeleton-line skeleton-line--meta"></span>
      </span>
      <span class="skeleton-pill"></span>
    </div>
  {/each}
</div>

<style>
  .skeleton-list {
    display: flex;
    flex-direction: column;
    gap: 3px;
    padding-top: 3px;
  }

  .skeleton-row {
    display: flex;
    min-height: 42px;
    align-items: center;
    gap: 9px;
    padding: 4px;
    border-radius: 5px;
    background: color-mix(in srgb, var(--surface-2) 26%, transparent);
  }

  .skeleton-row.is-compact {
    min-height: 34px;
  }

  .skeleton-art {
    width: 34px;
    height: 34px;
    flex: 0 0 34px;
    border-radius: 5px;
    background: var(--surface-2);
  }

  .skeleton-row.is-compact .skeleton-art {
    width: 28px;
    height: 28px;
    flex-basis: 28px;
  }

  .skeleton-copy {
    display: flex;
    flex: 1 1 auto;
    flex-direction: column;
    gap: 5px;
  }

  .skeleton-line,
  .skeleton-art,
  .skeleton-pill {
    position: relative;
    overflow: hidden;
    background: var(--surface-2);
  }

  .skeleton-line::after,
  .skeleton-art::after,
  .skeleton-pill::after {
    content: "";
    position: absolute;
    inset: 0;
    background: linear-gradient(
      90deg,
      transparent,
      color-mix(in srgb, var(--text) 8%, transparent),
      transparent
    );
    transform: translateX(-100%);
    animation: skeleton-sheen 1.4s ease-in-out infinite;
  }

  .skeleton-line {
    height: 7px;
    border-radius: 999px;
  }

  .skeleton-line--title {
    width: 46%;
  }

  .skeleton-line--meta {
    width: 28%;
    height: 5px;
    opacity: 0.65;
  }

  .skeleton-pill {
    width: 28px;
    height: 6px;
    flex: 0 0 28px;
    border-radius: 999px;
    opacity: 0.65;
  }

  @keyframes skeleton-sheen {
    0% {
      transform: translateX(-100%);
    }
    60%,
    100% {
      transform: translateX(100%);
    }
  }
</style>
