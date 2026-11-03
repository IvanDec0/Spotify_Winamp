<script lang="ts">
  import { player } from "../../player.svelte";
  import { formatTime } from "../../format";

  interface Props {
    maxItems?: number;
    showHeader?: boolean;
    showArt?: boolean;
    showTime?: boolean;
  }

  let { maxItems = 20, showHeader = true, showArt = true, showTime = true }: Props = $props();

  let items = $derived((player.state.queue ?? []).slice(0, Math.max(1, maxItems)));
  let countLabel = $derived(items.length.toString().padStart(2, "0"));
</script>

<section class="cad-queue" aria-label="Playback queue" data-component="queue">
  {#if showHeader}
    <header class="cad-queue-head">
      <div>
        <span class="cad-queue-title">Up next</span>
        <span class="cad-queue-count">{countLabel}</span>
      </div>
      <span class="cad-queue-note">Simulated queue</span>
    </header>
  {/if}

  {#if items.length === 0}
    <p class="cad-queue-empty">Nothing queued. Start a track in Spotify.</p>
  {:else}
    <ol class="cad-queue-list">
      {#each items as item (item.index)}
        <li
          class="cad-queue-row"
          class:is-current={item.isCurrent}
          class:is-next={item.isNext}
          aria-current={item.isCurrent ? "true" : undefined}
        >
          <span class="cad-queue-index">
            {#if item.isCurrent}
              <span aria-label="Currently playing">▶</span>
            {:else}
              {String(item.index + 1).padStart(2, "0")}
            {/if}
          </span>

          {#if showArt}
            <span class={`cad-queue-art cad-queue-art--${item.track.cover}`} aria-hidden="true">
              <svg viewBox="0 0 24 24" fill="none">
                <circle
                  cx="12"
                  cy="12"
                  r="6.5"
                  stroke="currentColor"
                  stroke-width="1"
                  opacity=".6"
                />
                <circle cx="12" cy="12" r="2" fill="currentColor" />
              </svg>
            </span>
          {/if}

          <span class="cad-queue-copy">
            <span class="cad-queue-title" title={item.track.title}>{item.track.title}</span>
            <span class="cad-queue-artist" title={item.track.artists?.join(", ") ?? ""}>
              {item.track.artists?.join(", ") || "Unknown artist"}
            </span>
          </span>

          {#if showTime}
            <span class="cad-queue-time">{formatTime(item.track.durationMs / 1000)}</span>
          {/if}
        </li>
      {/each}
    </ol>
  {/if}
</section>
