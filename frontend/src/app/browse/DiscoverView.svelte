<script lang="ts">
  import { onMount } from "svelte";
  import { coverURL } from "../../lib/artwork";
  import { t, type MessageKey } from "../../shared/i18n";
  import Button from "../../shared/ui/Button.svelte";
  import Skeleton from "../../shared/ui/Skeleton.svelte";
  import Icon from "./Icon.svelte";
  import type { BrowseIcon } from "./icons";
  import {
    browse,
    entityFromLibraryEntry,
    type EntityRef,
    type LibraryEntry,
    type SearchTab,
  } from "./browse.svelte";

  interface GenreCard {
    query: string;
    tab: SearchTab;
    labelKey: MessageKey;
    from: string;
    to: string;
  }

  const GENRES: GenreCard[] = [
    { query: "Pop", tab: "tracks", labelKey: "browse.genre.pop", from: "#ff5c8a", to: "#ff9966" },
    { query: "Rock", tab: "tracks", labelKey: "browse.genre.rock", from: "#64748b", to: "#1f2937" },
    {
      query: "Hip-Hop",
      tab: "tracks",
      labelKey: "browse.genre.hipHop",
      from: "#f59e0b",
      to: "#b45309",
    },
    {
      query: "Electronic",
      tab: "tracks",
      labelKey: "browse.genre.electronic",
      from: "#22d3ee",
      to: "#6366f1",
    },
    { query: "Jazz", tab: "tracks", labelKey: "browse.genre.jazz", from: "#a78bfa", to: "#4c1d95" },
    {
      query: "Classical",
      tab: "tracks",
      labelKey: "browse.genre.classical",
      from: "#64748b",
      to: "#334155",
    },
    {
      query: "Latin",
      tab: "tracks",
      labelKey: "browse.genre.latin",
      from: "#fb7185",
      to: "#f97316",
    },
    { query: "R&B", tab: "tracks", labelKey: "browse.genre.rnb", from: "#c084fc", to: "#db2777" },
    {
      query: "Indie",
      tab: "tracks",
      labelKey: "browse.genre.indie",
      from: "#34d399",
      to: "#0f766e",
    },
    {
      query: "Chill",
      tab: "tracks",
      labelKey: "browse.genre.chill",
      from: "#38bdf8",
      to: "#0ea5e9",
    },
    {
      query: "Party",
      tab: "tracks",
      labelKey: "browse.genre.party",
      from: "#f472b6",
      to: "#facc15",
    },
    {
      query: "Podcast",
      tab: "shows",
      labelKey: "browse.genre.podcasts",
      from: "#5ef0ff",
      to: "#7d8cff",
    },
  ];

  const LIKED_SONGS_URI = "spotify:collection:tracks";

  let discover = $derived(browse.discover);
  let libraryEmpty = $derived(
    discover.loaded &&
      !discover.loading &&
      !discover.error &&
      discover.playlists.length === 0 &&
      discover.albums.length === 0 &&
      discover.artists.length === 0,
  );

  onMount(() => {
    if (!browse.discover.loaded && !browse.discover.error) {
      void browse.ensureDiscover();
    }
  });

  function greetingKey(): MessageKey {
    const hour = new Date().getHours();
    if (hour < 12) return "browse.discover.greetingMorning";
    if (hour < 18) return "browse.discover.greetingAfternoon";
    return "browse.discover.greetingEvening";
  }

  let greeting = $derived(t(greetingKey()));

  function kindLabel(kind: EntityRef["kind"]): string {
    switch (kind) {
      case "album":
        return t("browse.album");
      case "artist":
        return t("browse.artist");
      case "show":
        return t("browse.show");
      default:
        return t("browse.playlist");
    }
  }

  function entityIcon(kind: EntityRef["kind"]): BrowseIcon {
    switch (kind) {
      case "artist":
        return "library";
      case "show":
        return "podcasts";
      case "playlist":
        return "library";
      default:
        return "note";
    }
  }

  function entityTitle(ref: EntityRef): string {
    return ref.uri === LIKED_SONGS_URI ? t("browse.likedSongs") : ref.title;
  }

  function entityMeta(ref: EntityRef): string {
    if (ref.uri === LIKED_SONGS_URI) return t("browse.likedSongsHint");
    return ref.subtitle || ref.owner || kindLabel(ref.kind);
  }

  function entryIcon(entry: LibraryEntry): BrowseIcon {
    switch (entry.kind) {
      case "artist":
        return "library";
      case "episode":
        return "podcasts";
      case "playlist":
        return "library";
      default:
        return "note";
    }
  }

  function entryMeta(entry: LibraryEntry): string {
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
        return entry.artists.join(", ") || t("browse.podcast");
    }
  }

  function openEntry(entry: LibraryEntry): void {
    if (entry.kind === "episode") {
      void browse.playEntry(entry);
      return;
    }
    const entity = entityFromLibraryEntry(entry);
    if (entity) void browse.openEntity(entity);
  }

  function focusSearch(): void {
    document.getElementById("browse-search-input")?.focus();
  }

  function runRecentSearch(query: string): void {
    browse.recordSearch(query);
    void browse.search(query);
  }

  function searchGenre(genre: GenreCard): void {
    void browse.searchGenre({ query: genre.query, tab: genre.tab });
  }

  function searchPodcasts(): void {
    void browse.searchGenre({ query: "Podcast", tab: "shows" });
  }
</script>

{#snippet libraryCard(entry: LibraryEntry)}
  <button class="discover-card" type="button" onclick={() => openEntry(entry)}>
    <span class="discover-art" class:is-round={entry.kind === "artist"}>
      {#if entry.image}
        <img
          src={coverURL(entry.image, 160)}
          alt=""
          width="150"
          height="150"
          loading="lazy"
          decoding="async"
        />
      {:else}
        <Icon name={entryIcon(entry)} size={22} />
      {/if}
    </span>
    <span class="discover-card-title">{entry.title}</span>
    <span class="discover-card-meta">{entryMeta(entry)}</span>
  </button>
{/snippet}

<div class="discover">
  <header class="discover-hero">
    <h2 class="discover-title">{greeting}</h2>
    <p class="discover-subtitle">{t("browse.discover.subtitle")}</p>
  </header>

  <section class="discover-section" aria-labelledby="discover-favorites-title">
    <h2 id="discover-favorites-title" class="discover-heading">
      {t("browse.discover.favorites")}
    </h2>
    <div class="quick-grid">
      <button class="quick-card" type="button" onclick={() => browse.openLikedSongs()}>
        <span class="quick-icon" aria-hidden="true"><Icon name="heart" size={18} filled /></span>
        <span class="quick-copy">
          <span class="quick-title">{t("browse.likedSongs")}</span>
          <span class="quick-meta">{t("browse.likedSongsHint")}</span>
        </span>
      </button>
      <button class="quick-card" type="button" onclick={() => browse.openYourEpisodes()}>
        <span class="quick-icon is-podcast" aria-hidden="true">
          <Icon name="podcasts" size={18} />
        </span>
        <span class="quick-copy">
          <span class="quick-title">{t("browse.yourEpisodes")}</span>
          <span class="quick-meta">{t("browse.discover.yourEpisodesHint")}</span>
        </span>
      </button>
    </div>
  </section>

  {#if browse.recentEntities.length > 0}
    <section class="discover-section" aria-labelledby="discover-recent-title">
      <h2 id="discover-recent-title" class="discover-heading">
        {t("browse.discover.recent")}
      </h2>
      <div class="discover-row">
        {#each browse.recentEntities as ref (ref.uri)}
          <button class="discover-card" type="button" onclick={() => void browse.openEntity(ref)}>
            <span class="discover-art" class:is-round={ref.kind === "artist"}>
              {#if ref.image}
                <img
                  src={coverURL(ref.image, 160)}
                  alt=""
                  width="150"
                  height="150"
                  loading="lazy"
                  decoding="async"
                />
              {:else}
                <Icon name={entityIcon(ref.kind)} size={22} />
              {/if}
            </span>
            <span class="discover-card-title">{entityTitle(ref)}</span>
            <span class="discover-card-meta">{entityMeta(ref)}</span>
          </button>
        {/each}
      </div>
    </section>
  {/if}

  {#if discover.error}
    <div class="browse-banner is-error" role="alert" aria-live="assertive">
      <span>{t("browse.discover.loadError")} {discover.error}</span>
      <Button variant="ghost" size="sm" onclick={() => void browse.ensureDiscover()}>
        {t("ui.retry")}
      </Button>
    </div>
  {/if}

  {#if discover.loading && !discover.loaded}
    <div
      class="skeleton-wrap discover-skeleton"
      aria-busy="true"
      aria-label={t("browse.discover.loading")}
    >
      <Skeleton rows={6} height={18} />
    </div>
  {:else if discover.loaded}
    <section class="discover-section" aria-labelledby="discover-library-title">
      <h2 id="discover-library-title" class="discover-heading">
        {t("browse.discover.library")}
      </h2>
      {#if libraryEmpty}
        <div class="empty-panel">
          <Icon name="library" size={22} />
          <p>{t("browse.discover.emptyLibrary")}</p>
          <Button variant="secondary" size="sm" onclick={focusSearch}>
            <Icon name="search" size={13} />
            {t("browse.discover.searchCatalog")}
          </Button>
        </div>
      {:else}
        {#if discover.playlists.length > 0}
          <div class="discover-group">
            <h3 class="discover-subheading">{t("browse.playlists")}</h3>
            <div class="discover-row">
              {#each discover.playlists as entry (entry.uri)}
                {@render libraryCard(entry)}
              {/each}
            </div>
          </div>
        {/if}
        {#if discover.albums.length > 0}
          <div class="discover-group">
            <h3 class="discover-subheading">{t("browse.albums")}</h3>
            <div class="discover-row">
              {#each discover.albums as entry (entry.uri)}
                {@render libraryCard(entry)}
              {/each}
            </div>
          </div>
        {/if}
        {#if discover.artists.length > 0}
          <div class="discover-group">
            <h3 class="discover-subheading">{t("browse.artists")}</h3>
            <div class="discover-row">
              {#each discover.artists as entry (entry.uri)}
                {@render libraryCard(entry)}
              {/each}
            </div>
          </div>
        {/if}
      {/if}
    </section>

    <section class="discover-section" aria-labelledby="discover-podcasts-title">
      <div class="discover-section-head">
        <h2 id="discover-podcasts-title" class="discover-heading">
          {t("browse.discover.podcasts")}
        </h2>
        {#if discover.episodes.length === 0}
          <Button variant="secondary" size="sm" onclick={searchPodcasts}>
            <Icon name="search" size={13} />
            {t("browse.searchPodcasts")}
          </Button>
        {/if}
      </div>
      {#if discover.episodes.length === 0}
        <p class="discover-empty-copy">{t("browse.discover.emptyPodcasts")}</p>
      {:else}
        <div class="discover-row">
          {#each discover.episodes as entry (entry.uri)}
            {@render libraryCard(entry)}
          {/each}
        </div>
      {/if}
    </section>
  {/if}

  <section class="discover-section" aria-labelledby="discover-genres-title">
    <h2 id="discover-genres-title" class="discover-heading">{t("browse.discover.genres")}</h2>
    <div class="genre-grid">
      {#each GENRES as genre (genre.labelKey)}
        <button
          class="genre-card"
          type="button"
          style={`--genre-from:${genre.from};--genre-to:${genre.to};`}
          onclick={() => searchGenre(genre)}
        >
          <span class="genre-label">{t(genre.labelKey)}</span>
        </button>
      {/each}
    </div>
  </section>

  {#if browse.recentSearches.length > 0}
    <section class="discover-section" aria-labelledby="discover-searches-title">
      <div class="discover-section-head">
        <h2 id="discover-searches-title" class="discover-heading">
          {t("browse.discover.recentSearches")}
        </h2>
        <Button variant="ghost" size="sm" onclick={() => browse.clearRecentSearches()}>
          {t("browse.discover.clearRecent")}
        </Button>
      </div>
      <div class="recent-searches">
        {#each browse.recentSearches as query (query)}
          <button class="recent-chip" type="button" onclick={() => runRecentSearch(query)}>
            <Icon name="search" size={12} />
            <span>{query}</span>
          </button>
        {/each}
      </div>
    </section>
  {/if}
</div>
