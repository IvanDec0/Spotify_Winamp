<script lang="ts">
  import type { SearchHit } from "../../shared/api/types";
  import Icon from "./Icon.svelte";
  import MediaCover from "./MediaCover.svelte";

  interface Props {
    hit: SearchHit;
    kind: "track" | "album" | "artist" | "playlist";
    meta: string;
    onOpen: () => void;
    onRadio: () => void;
    onQueue?: () => void;
  }

  let { hit, kind, meta, onOpen, onRadio, onQueue }: Props = $props();
</script>

<li class="search-row">
  <button class="search-main" type="button" onclick={onOpen}>
    <MediaCover src={hit.image} label={hit.name || hit.uri} size={34} round={kind === "artist"} />
    <span class="search-copy">
      <span class="search-name">{hit.name || hit.uri}</span>
      <span class="search-meta">{meta}</span>
    </span>
  </button>
  <span class="search-actions">
    {#if kind === "track" && onQueue}
      <button
        class="search-action"
        type="button"
        title="Add to play queue"
        aria-label={`Add ${hit.name || "track"} to queue`}
        onclick={onQueue}
      >
        +
      </button>
    {/if}
    <button
      class="search-action radio-action"
      type="button"
      title="Start radio"
      aria-label={`Start radio from ${hit.name || hit.uri}`}
      onclick={onRadio}
    >
      <Icon name="radio" size={10} />
    </button>
  </span>
</li>

<style>
  .search-row {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 2px 2px 2px 4px;
    border-radius: 5px;
    transition:
      background-color var(--motion-fast) ease,
      transform var(--motion-fast) var(--ease-out);
  }

  .search-row:hover {
    background: color-mix(in srgb, var(--accent) 7%, transparent);
    transform: translateX(1px);
  }

  .search-main {
    display: flex;
    min-width: 0;
    flex: 1 1 auto;
    align-items: center;
    gap: 8px;
    padding: 3px 2px;
    text-align: left;
  }

  .search-copy {
    display: flex;
    min-width: 0;
    flex: 1 1 auto;
    flex-direction: column;
    gap: 2px;
  }

  .search-name {
    overflow: hidden;
    color: var(--text);
    font-size: 9.5px;
    font-weight: 550;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .search-meta {
    overflow: hidden;
    color: var(--faint);
    font-size: 7.5px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .search-actions {
    display: flex;
    flex: 0 0 auto;
    align-items: center;
    gap: 3px;
  }

  .search-action {
    display: grid;
    width: 20px;
    height: 20px;
    place-items: center;
    border: 1px solid var(--line);
    border-radius: 4px;
    color: var(--faint);
    font-size: 12px;
    line-height: 1;
    transition:
      border-color var(--motion-fast) ease,
      color var(--motion-fast) ease,
      background-color var(--motion-fast) ease,
      transform var(--motion-fast) var(--ease-spring);
  }

  .search-action:hover {
    border-color: color-mix(in srgb, var(--accent) 38%, transparent);
    background: color-mix(in srgb, var(--accent) 8%, transparent);
    color: var(--accent);
    transform: translateY(-1px);
  }

  .radio-action {
    color: color-mix(in srgb, var(--accent) 72%, var(--muted));
  }
</style>
