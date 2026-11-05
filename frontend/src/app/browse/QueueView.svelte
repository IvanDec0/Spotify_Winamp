<script lang="ts">
  import type { QueueItem } from "../../shared/api/types";
  import { coverURL } from "../../lib/artwork";
  import { formatTime } from "../../lib/format";
  import { player } from "../../lib/player.svelte";
  import { t } from "../../shared/i18n";
  import Skeleton from "../../shared/ui/Skeleton.svelte";
  import Icon from "./Icon.svelte";

  let items = $derived(player.queue);
  let loading = $derived(player.queueLoading);
  let countLabel = $derived(t("browse.tracksCount", { count: items.length }));

  $effect(() => {
    const uri = player.state.track.uri;
    if (!uri) return;
    void player.loadQueue();
  });

  function coverImage(item: QueueItem): string {
    return item.track.cover.startsWith("/art/") ? coverURL(item.track.cover, 40) : "";
  }

  function artistLabel(item: QueueItem): string {
    return item.track.artists?.join(", ") || t("player.unknownArtist");
  }
</script>

<section class="browse-view queue-view" aria-labelledby="queue-view-title" aria-busy={loading}>
  <header class="view-head">
    <div>
      <p class="eyebrow">{t("queue.upNext")}</p>
      <h1 id="queue-view-title" class="view-title" tabindex="-1">
        {t("queue.playback")}
      </h1>
      <p class="view-subtitle">{t("browse.queueHint")}</p>
    </div>
    <span class="queue-total">{countLabel}</span>
  </header>

  {#if loading && items.length === 0}
    <p class="queue-view-loading" role="status">{t("browse.queueLoading")}</p>
    <div class="skeleton-wrap" aria-busy="true" aria-hidden="true">
      <Skeleton rows={7} height={16} />
    </div>
  {:else if items.length === 0}
    <div class="empty-panel">
      <Icon name="queue" size={22} />
      <p>{t("browse.queueEmpty")}</p>
    </div>
  {:else}
    <ol class="queue-view-list">
      {#each items as item (item.index)}
        <li
          class="queue-view-row"
          class:is-current={item.isCurrent}
          class:is-next={item.isNext}
          aria-current={item.isCurrent ? "true" : undefined}
        >
          <span class="queue-view-index">
            {#if item.isCurrent}
              <span class="queue-playing-bars" aria-label={t("queue.currentlyPlaying")}>
                <i></i><i></i><i></i>
              </span>
            {:else}
              {String(item.index + 1).padStart(2, "0")}
            {/if}
          </span>

          <span class="queue-view-cover">
            {#if coverImage(item)}
              <img
                src={coverImage(item)}
                alt=""
                width="40"
                height="40"
                loading="lazy"
                decoding="async"
              />
            {:else}
              <Icon name="note" size={15} />
            {/if}
          </span>

          <span class="queue-view-copy">
            <span class="queue-view-title" title={item.track.title}>{item.track.title}</span>
            <span class="queue-view-meta" title={item.track.artists?.join(", ") ?? ""}>
              {artistLabel(item)}
            </span>
          </span>

          <span class="queue-view-tags">
            {#if item.isCurrent}
              <span class="queue-view-tag is-current">{t("browse.nowPlaying")}</span>
            {/if}
            {#if item.isNext}
              <span class="queue-view-tag">{t("queue.next")}</span>
            {/if}
          </span>

          <span class="queue-view-time">{formatTime(item.track.durationMs / 1000)}</span>
        </li>
      {/each}
    </ol>
  {/if}
</section>
