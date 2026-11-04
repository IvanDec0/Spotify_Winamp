<script lang="ts" generics="T">
  import type { Snippet } from "svelte";
  import { visibleRange } from "./virtual";

  interface Props<T> {
    items: T[];
    itemHeight: number;
    height?: number;
    overscan?: number;
    /**
     * Stable identity for each row. Consumers must pass this when items can
     * reorder; otherwise the index is used and row state follows the position.
     */
    key?: (item: T, index: number) => string | number;
    children: Snippet<[T, number]>;
  }

  let { items, itemHeight, height = 0, overscan = 4, key, children }: Props<T> = $props();

  let viewport: HTMLDivElement | undefined = $state();
  let measured = $state(0);
  let scrollTop = $state(0);

  let viewportHeight = $derived(height > 0 ? height : measured);
  let range = $derived(visibleRange(scrollTop, viewportHeight, itemHeight, items.length, overscan));
  let windowItems = $derived.by(() =>
    items.slice(range.start, range.end).map((item, offset) => {
      const index = range.start + offset;
      return { item, index, key: key ? key(item, index) : index };
    }),
  );

  $effect(() => {
    if (height > 0 || !viewport) return;
    const update = (): void => {
      if (viewport) measured = viewport.clientHeight;
    };
    update();
    if (typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(update);
    observer.observe(viewport);
    return () => observer.disconnect();
  });

  function handleScroll(event: Event): void {
    scrollTop = (event.currentTarget as HTMLDivElement).scrollTop;
  }
</script>

<div
  class="ui-virtual-list"
  style:height={height > 0 ? `${height}px` : undefined}
  bind:this={viewport}
  onscroll={handleScroll}
>
  <div class="ui-virtual-spacer" style:height={`${items.length * itemHeight}px`}>
    <div class="ui-virtual-window" style:transform={`translateY(${range.start * itemHeight}px)`}>
      {#each windowItems as entry (entry.key)}
        {@render children(entry.item, entry.index)}
      {/each}
    </div>
  </div>
</div>

<style>
  .ui-virtual-list {
    overflow-y: auto;
    overscroll-behavior: contain;
  }

  .ui-virtual-spacer {
    position: relative;
  }

  .ui-virtual-window {
    position: absolute;
    top: 0;
    right: 0;
    left: 0;
    will-change: transform;
  }
</style>
