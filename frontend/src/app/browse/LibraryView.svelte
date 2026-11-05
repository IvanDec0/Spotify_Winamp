<script lang="ts">
  import { onMount, tick } from "svelte";
  import { coverURL } from "../../lib/artwork";
  import { formatTime } from "../../lib/format";
  import { player } from "../../lib/player.svelte";
  import { t } from "../../shared/i18n";
  import Button from "../../shared/ui/Button.svelte";
  import Skeleton from "../../shared/ui/Skeleton.svelte";
  import Icon from "./Icon.svelte";
  import {
    browse,
    entityFromLibraryEntry,
    type LibraryEntry,
    type LibraryTab,
  } from "./browse.svelte";
  import { nextTabIndex, revealTab } from "./tablist";

  const TABS: ReadonlyArray<{ id: LibraryTab; label: Parameters<typeof t>[0] }> = [
    { id: "playlists", label: "browse.playlists" },
    { id: "albums", label: "browse.albums" },
    { id: "artists", label: "browse.artists" },
    { id: "episodes", label: "browse.yourEpisodes" },
  ];

  let page = $derived(browse.pages[browse.libraryTab]);
  let canLoadMore = $derived(
    browse.libraryTab !== "episodes" && page.loaded && page.items.length < page.total,
  );

  onMount(() => {
    if (!page.loaded && !page.loading) void browse.loadLibrary(browse.libraryTab);
  });

  function meta(entry: LibraryEntry): string {
    switch (entry.kind) {
      case "playlist":
        return `${entry.owner || t("browse.playlist")} · ${t("browse.tracksCount", {
          count: entry.length,
        })}`;
      case "album":
        return [entry.artists.join(", ") || t("browse.album"), entry.year || ""]
          .filter(Boolean)
          .join(" · ");
      case "artist":
        return t("browse.artist");
      default:
        return [
          entry.owner,
          entry.artists.join(", "),
          entry.durationMs > 0 ? formatTime(entry.durationMs / 1000) : "",
        ]
          .filter(Boolean)
          .join(" · ");
    }
  }

  function openEntry(entry: LibraryEntry): void {
    if (entry.kind === "episode") {
      void browse.playEpisode(entry.uri);
      return;
    }
    const entity = entityFromLibraryEntry(entry);
    if (entity) void browse.openEntity(entity);
  }

  function openLikedSongs(): void {
    void browse.openEntity({
      kind: "playlist",
      uri: "spotify:collection:tracks",
      title: t("browse.likedSongs"),
      canEdit: false,
    });
  }

  $effect(() => {
    const id = browse.libraryTab;
    void tick().then(() => {
      document
        .getElementById(`library-tab-${id}`)
        ?.scrollIntoView?.({ block: "nearest", inline: "nearest" });
    });
  });

  function handleTabKey(event: KeyboardEvent, index: number): void {
    const next = nextTabIndex(event.key, index, TABS.length);
    if (next === null) return;
    event.preventDefault();
    const tab = TABS[next];
    browse.setLibraryTab(tab.id);
    void tick().then(() => revealTab(`library-tab-${tab.id}`));
  }

  function retryLibrary(): void {
    void browse.loadLibrary(browse.libraryTab);
  }
</script>

<section class="browse-view" aria-labelledby="library-view-title">
  <header class="view-head">
    <div>
      <p class="eyebrow">{t("browse.library")}</p>
      <h1 id="library-view-title" class="view-title" tabindex="-1">
        {t("browse.libraryTitle")}
      </h1>
    </div>
  </header>

  <button class="liked-card" type="button" onclick={openLikedSongs}>
    <span class="liked-mark" aria-hidden="true">
      <Icon name="heart" size={20} filled />
    </span>
    <span class="result-copy">
      <span class="result-title">{t("browse.likedSongs")}</span>
      <span class="result-meta">{t("browse.likedSongsHint")}</span>
    </span>
    <span class="row-arrow"><Icon name="external" size={13} /></span>
  </button>

  {#if browse.libraryError}
    <div class="browse-banner is-error" role="alert" aria-live="assertive">
      <span>{browse.libraryError}</span>
      <Button variant="ghost" size="sm" onclick={retryLibrary}>{t("ui.retry")}</Button>
    </div>
  {/if}

  <div class="tab-bar" role="tablist" aria-label={t("browse.library")}>
    {#each TABS as tab, index (tab.id)}
      <button
        class="tab"
        class:is-active={browse.libraryTab === tab.id}
        id={`library-tab-${tab.id}`}
        role="tab"
        type="button"
        aria-selected={browse.libraryTab === tab.id}
        aria-controls="library-panel"
        tabindex={browse.libraryTab === tab.id ? 0 : -1}
        onclick={() => browse.setLibraryTab(tab.id)}
        onkeydown={(event) => handleTabKey(event, index)}
      >
        {t(tab.label)}
      </button>
    {/each}
  </div>

  <div
    id="library-panel"
    class="tab-panel"
    role="tabpanel"
    aria-labelledby={`library-tab-${browse.libraryTab}`}
    aria-busy={page.loading}
  >
    {#if page.loading && page.items.length === 0}
      <div class="skeleton-wrap" aria-busy="true" aria-label={t("browse.loading")}>
        <Skeleton rows={7} height={16} />
      </div>
    {:else if page.items.length === 0 && !browse.libraryError}
      <p class="empty-panel">{t("browse.emptyLibrary")}</p>
    {:else if page.items.length > 0}
      <ul class="result-list">
        {#each page.items as entry (entry.uri)}
          <li
            class="result-row"
            class:is-current={entry.kind === "episode" && entry.uri === player.state.track.uri}
          >
            <button class="result-main" type="button" onclick={() => openEntry(entry)}>
              <span class="result-cover" class:is-round={entry.kind === "artist"}>
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
                  <Icon name="note" size={16} />
                {/if}
              </span>
              <span class="result-copy">
                <span class="result-title">{entry.title}</span>
                <span class="result-meta">{meta(entry)}</span>
              </span>
            </button>
            {#if entry.kind === "episode"}
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
              </div>
            {:else}
              <span class="row-arrow"><Icon name="external" size={13} /></span>
            {/if}
          </li>
        {/each}
      </ul>

      {#if canLoadMore}
        <div class="load-more">
          <Button
            variant="secondary"
            size="sm"
            disabled={page.loading}
            onclick={() => void browse.loadMore(browse.libraryTab)}
          >
            {page.loading ? t("browse.loading") : t("browse.loadMore")}
          </Button>
        </div>
      {/if}
    {/if}
  </div>
</section>
