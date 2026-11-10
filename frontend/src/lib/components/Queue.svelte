<script lang="ts">
  import type { QueueItem } from "../../shared/api/types";
  import { t } from "../../shared/i18n";
  import { formatTime } from "../format";
  import CoverArt from "./CoverArt.svelte";
  import Icon from "./Icon.svelte";

  interface Props {
    items: QueueItem[];
    provider?: string;
    loading?: boolean;
  }

  let { items, provider = "demo", loading = false }: Props = $props();

  let countLabel = $derived(items.length.toString().padStart(2, "0"));
  let queueNote = $derived(provider === "demo" ? t("queue.demo") : t("queue.playback"));
</script>

<section class="queue" aria-label={t("queue.playback")} aria-busy={loading}>
  <header class="queue-head">
    <div class="queue-heading">
      <span class="eyebrow">{t("queue.upNext")}</span>
      <span class="queue-count mono">{countLabel}</span>
    </div>
    <span class="queue-note mono">{queueNote}</span>
  </header>

  {#if items.length === 0}
    <div class="queue-empty-state">
      <span class="queue-empty-icon" aria-hidden="true"><Icon name="queue" size={16} /></span>
      {#if loading}
        <p class="queue-empty" role="status">{t("ui.loading")}</p>
      {:else}
        <p class="queue-empty">{t("queue.empty")}</p>
      {/if}
    </div>
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
              <span class="playing-bars" aria-label={t("queue.currentlyPlaying")}>
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
              {item.track.artists?.join(", ") || t("player.unknownArtist")}
            </span>
          </span>

          {#if item.isNext}
            <span class="next-tag mono">{t("queue.next")}</span>
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
      linear-gradient(180deg, color-mix(in srgb, var(--surface) 52%, transparent), transparent 26%),
      color-mix(in srgb, var(--bg) 78%, transparent);
    backdrop-filter: blur(18px);
  }

  .queue-head {
    position: relative;
    display: flex;
    flex: 0 0 auto;
    align-items: baseline;
    justify-content: space-between;
    padding-bottom: 7px;
    border-bottom: 1px solid var(--line);
  }

  .queue-head::after {
    content: "";
    position: absolute;
    bottom: -1px;
    left: 0;
    width: 28px;
    height: 1px;
    background: linear-gradient(90deg, var(--accent), transparent);
    box-shadow: 0 0 7px var(--accent-glow);
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
    border-radius: 6px;
    color: var(--muted);
    animation: queue-row-in 360ms var(--ease-out) both;
    transition:
      background-color var(--motion-fast) ease,
      transform var(--motion-fast) var(--ease-out);
  }

  .queue-row:nth-child(2) {
    animation-delay: 30ms;
  }

  .queue-row:nth-child(3) {
    animation-delay: 60ms;
  }

  .queue-row:nth-child(4) {
    animation-delay: 90ms;
  }

  .queue-row:nth-child(5) {
    animation-delay: 120ms;
  }

  .queue-row:nth-child(6) {
    animation-delay: 150ms;
  }

  .queue-row:nth-child(n + 7) {
    animation-delay: 180ms;
  }

  @keyframes queue-row-in {
    from {
      opacity: 0;
      transform: translateX(8px);
    }
    to {
      opacity: 1;
      transform: translateX(0);
    }
  }

  .queue-row:hover {
    background: color-mix(in srgb, var(--surface-2) 68%, transparent);
    transform: translateX(2px);
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

  .queue-empty-state {
    display: flex;
    flex: 1 1 auto;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 9px;
    padding: 16px 8px;
    animation: queue-empty-in 420ms var(--ease-out) both;
  }

  .queue-empty-icon {
    display: grid;
    width: 34px;
    height: 34px;
    place-items: center;
    border: 1px solid var(--line);
    border-radius: 50%;
    background: color-mix(in srgb, var(--surface-2) 55%, transparent);
    color: var(--faint);
    box-shadow: 0 0 0 4px color-mix(in srgb, var(--accent) 4%, transparent);
  }

  .queue-empty {
    margin: 0;
    color: var(--faint);
    font-size: 10px;
    text-align: center;
  }

  @keyframes queue-empty-in {
    from {
      opacity: 0;
      transform: translateY(6px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
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
