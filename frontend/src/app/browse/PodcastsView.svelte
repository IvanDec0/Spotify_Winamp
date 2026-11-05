<script lang="ts">
  import { onMount } from "svelte";
  import { coverURL } from "../../lib/artwork";
  import { formatTime } from "../../lib/format";
  import { t } from "../../shared/i18n";
  import Button from "../../shared/ui/Button.svelte";
  import Skeleton from "../../shared/ui/Skeleton.svelte";
  import Icon from "./Icon.svelte";
  import { browse, type LibraryEntry } from "./browse.svelte";

  let page = $derived(browse.pages.episodes);

  onMount(() => {
    if (!page.loaded && !page.loading) void browse.loadLibrary("episodes");
  });

  function meta(entry: LibraryEntry): string {
    return [
      entry.owner,
      entry.artists.join(", "),
      entry.durationMs > 0 ? formatTime(entry.durationMs / 1000) : "",
    ]
      .filter(Boolean)
      .join(" · ");
  }

  function openShow(entry: LibraryEntry): void {
    if (!entry.showUri) return;
    void browse.openEntity({
      kind: "show",
      uri: entry.showUri,
      title: entry.owner || entry.title,
      image: entry.image,
    });
  }
</script>

<section class="browse-view" aria-labelledby="podcasts-view-title">
  <header class="view-head">
    <div>
      <p class="eyebrow">{t("browse.podcasts")}</p>
      <h1 id="podcasts-view-title" class="view-title" tabindex="-1">
        {t("browse.yourEpisodes")}
      </h1>
      <p class="view-subtitle">{t("browse.podcastsHint")}</p>
    </div>
    <Button
      variant="secondary"
      size="sm"
      onclick={() => {
        browse.setSection("search");
        browse.setSearchTab("shows");
      }}
    >
      <Icon name="search" size={13} />
      {t("browse.searchPodcasts")}
    </Button>
  </header>

  <p class="podcast-note">{t("browse.savedShowsNote")}</p>

  {#if browse.libraryError}
    <div class="browse-banner is-error" role="alert" aria-live="assertive">
      <span>{browse.libraryError}</span>
      <Button variant="ghost" size="sm" onclick={() => void browse.loadLibrary("episodes")}>
        {t("ui.retry")}
      </Button>
    </div>
  {/if}

  <div class="podcasts-body" aria-busy={page.loading}>
    {#if page.loading && page.items.length === 0}
      <div class="skeleton-wrap" aria-busy="true" aria-label={t("browse.loading")}>
        <Skeleton rows={6} height={16} />
      </div>
    {:else if page.items.length === 0 && !browse.libraryError}
      <p class="empty-panel">{t("browse.episodesEmpty")}</p>
    {:else if page.items.length > 0}
      <ul class="result-list">
        {#each page.items as entry (entry.uri)}
          <li class="result-row">
            <button
              class="result-main"
              type="button"
              onclick={() => void browse.playEpisode(entry.uri)}
            >
              <span class="result-cover">
                {#if entry.image}
                  <img
                    src={coverURL(entry.image, 40)}
                    alt=""
                    width="40"
                    height="40"
                    loading="lazy"
                    decoding="async"
                  />
                {:else}
                  <Icon name="podcasts" size={16} />
                {/if}
              </span>
              <span class="result-copy">
                <span class="result-title">{entry.title}</span>
                <span class="result-meta">{meta(entry)}</span>
              </span>
            </button>
            <div class="row-actions">
              <button
                class="icon-button"
                type="button"
                title={t("browse.playNamed", { title: entry.title })}
                aria-label={t("browse.playNamed", { title: entry.title })}
                onclick={() => void browse.playEpisode(entry.uri)}
              >
                <Icon name="play" size={13} filled />
              </button>
              {#if entry.showUri}
                <button
                  class="icon-button"
                  type="button"
                  title={t("browse.openNamed", { title: entry.owner || entry.title })}
                  aria-label={t("browse.openNamed", { title: entry.owner || entry.title })}
                  onclick={() => openShow(entry)}
                >
                  <Icon name="external" size={13} />
                </button>
              {/if}
            </div>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
</section>
