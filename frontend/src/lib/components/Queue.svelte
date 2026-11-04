<script lang="ts">
  import type { QueueItem } from "../../shared/api/types";
  import { formatTime } from "../format";
  import CoverArt from "./CoverArt.svelte";

  interface Props {
    items: QueueItem[];
    provider?: string;
  }

  let { items, provider = "demo" }: Props = $props();

  let countLabel = $derived(items.length.toString().padStart(2, "0"));
  let queueNote = $derived(provider === "demo" ? "Demo queue" : "Local queue");
</script>

<section class="queue" aria-label="Playback queue">
  <header class="queue-head">
    <div class="queue-heading">
      <span class="eyebrow">Up next</span>
      <span class="queue-count mono">{countLabel}</span>
    </div>
    <span class="queue-note mono">{queueNote}</span>
  </header>

  {#if items.length === 0}
    <p class="queue-empty">Nothing queued. Start a track in the Library.</p>
  {:else}
    <ol class="queue-list">
      {#each items as item (item.index)}
        <li
          class="queue-row"
          class:is-current={item.isCurrent}
          class:is-next={item.isNext}
          aria-current={item.isCurrent ? "true" : undefined}
        >
          <span class="queue-index mono">
            {#if item.isCurrent}
              <span class="playing-bars" aria-label="Currently playing">
                <i></i><i></i><i></i>
              </span>
            {:else}
              {String(item.index + 1).padStart(2, "0")}
            {/if}
          </span>

          <CoverArt cover={item.track.cover} title={item.track.title} size={29} />

          <span class="queue-copy">
            <span class="queue-title" title={item.track.title}>{item.track.title}</span>
            <span class="queue-artist" title={item.track.artists?.join(", ") ?? ""}>
              {item.track.artists?.join(", ") || "Unknown artist"}
            </span>
          </span>

          {#if item.isNext}
            <span class="next-tag mono">Next</span>
          {/if}
          <span class="queue-time mono">{formatTime(item.track.durationMs / 1000)}</span>
        </li>
      {/each}
    </ol>
  {/if}
</section>

<style>
  .queue {
    display: flex;
    height: 100%;
    flex-direction: column;
    padding: 10px 14px 10px;
    border-top: 1px solid var(--line-strong);
    background:
      linear-gradient(180deg, color-mix(in srgb, var(--surface) 45%, transparent), transparent 30%),
      color-mix(in srgb, var(--bg) 88%, transparent);
  }

  .queue-head {
    display: flex;
    flex: 0 0 auto;
    align-items: baseline;
    justify-content: space-between;
    padding-bottom: 7px;
    border-bottom: 1px solid var(--line);
  }

  .queue-heading {
    display: flex;
    align-items: baseline;
    gap: 7px;
  }

  .eyebrow {
    color: var(--muted);
    font-family: "JetBrains Mono", monospace;
    font-size: 7.5px;
    font-weight: 500;
    letter-spacing: 0.2em;
    text-transform: uppercase;
  }

  .queue-count {
    padding: 1px 4px;
    border: 1px solid var(--line);
    border-radius: 3px;
    color: var(--muted);
    font-size: 7.5px;
    letter-spacing: 0.08em;
  }

  .queue-note {
    color: var(--faint);
    font-size: 6.5px;
    letter-spacing: 0.12em;
    text-transform: uppercase;
  }

  .queue-list {
    flex: 1 1 auto;
    min-height: 0;
    margin: 4px -6px 0 0;
    padding: 0 6px 0 0;
    overflow-y: auto;
    list-style: none;
    -webkit-mask-image: linear-gradient(
      180deg,
      transparent 0,
      black 7px,
      black calc(100% - 8px),
      transparent
    );
    mask-image: linear-gradient(
      180deg,
      transparent 0,
      black 7px,
      black calc(100% - 8px),
      transparent
    );
  }

  .queue-row {
    position: relative;
    display: grid;
    grid-template-columns: 22px 29px minmax(0, 1fr) auto auto;
    gap: 8px;
    align-items: center;
    min-height: 36px;
    padding: 3px 6px 3px 4px;
    border-radius: 5px;
    color: var(--muted);
    transition:
      background-color var(--motion-fast) ease,
      transform var(--motion-fast) var(--ease-out);
  }

  .queue-row:hover {
    background: color-mix(in srgb, var(--surface-2) 62%, transparent);
    transform: translateX(1px);
  }

  .queue-row.is-current {
    background:
      linear-gradient(90deg, color-mix(in srgb, var(--accent) 10%, transparent), transparent 78%),
      color-mix(in srgb, var(--surface-2) 40%, transparent);
    color: var(--text);
    box-shadow: 0 0 18px color-mix(in srgb, var(--accent) 5%, transparent) inset;
  }

  .queue-row.is-current::before {
    content: "";
    position: absolute;
    top: 9px;
    bottom: 9px;
    left: 0;
    width: 1.5px;
    border-radius: 2px;
    background: var(--accent);
    box-shadow: 0 0 7px var(--accent-glow);
  }

  .queue-index {
    display: grid;
    place-items: center;
    color: var(--faint);
    font-size: 7.5px;
    letter-spacing: 0.04em;
  }

  .queue-copy {
    display: flex;
    min-width: 0;
    flex-direction: column;
    gap: 2px;
  }

  .queue-title {
    overflow: hidden;
    color: color-mix(in srgb, var(--text) 88%, transparent);
    font-size: 10.5px;
    font-weight: 520;
    letter-spacing: -0.005em;
    line-height: 1.15;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .queue-row.is-current .queue-title {
    color: var(--text);
  }

  .queue-artist {
    overflow: hidden;
    color: var(--faint);
    font-size: 8.5px;
    line-height: 1.15;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .queue-row.is-current .queue-artist {
    color: color-mix(in srgb, var(--accent) 58%, var(--muted));
  }

  .queue-time {
    color: var(--faint);
    font-size: 7px;
    letter-spacing: 0.03em;
  }

  .next-tag {
    padding: 1px 4px;
    border: 1px solid var(--line);
    border-radius: 2px;
    color: var(--faint);
    font-size: 6px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .playing-bars {
    display: flex;
    height: 10px;
    align-items: flex-end;
    gap: 1.5px;
  }

  .playing-bars i {
    display: block;
    width: 1.5px;
    height: 5px;
    border-radius: 1px;
    background: var(--accent);
    box-shadow: 0 0 4px var(--accent-glow);
    animation: queue-bounce 900ms ease-in-out infinite;
  }

  .playing-bars i:nth-child(2) {
    height: 9px;
    animation-delay: -0.3s;
  }

  .playing-bars i:nth-child(3) {
    height: 7px;
    animation-delay: -0.6s;
  }

  .queue-empty {
    margin: auto;
    color: var(--faint);
    font-size: 10px;
  }

  @keyframes queue-bounce {
    0%,
    100% {
      transform: scaleY(0.4);
    }
    50% {
      transform: scaleY(1);
    }
  }
</style>
