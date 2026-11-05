<script lang="ts">
  import { formatTime } from "../../lib/format";
  import { t } from "../../shared/i18n";
  import Skeleton from "../../shared/ui/Skeleton.svelte";
  import { coverURL } from "../../lib/artwork";
  import { player } from "../../lib/player.svelte";
  import Icon from "./Icon.svelte";
  import { browse, type SearchTab } from "./browse.svelte";
  import type { SearchHit } from "../../shared/api/types";

  const TABS: ReadonlyArray<{ id: SearchTab; label: Parameters<typeof t>[0] }> = [
    { id: "tracks", label: "browse.tracks" },
    { id: "albums", label: "browse.albums" },
    { id: "artists", label: "browse.artists" },
    { id: "playlists", label: "browse.playlists" },
    { id: "shows", label: "browse.shows" },
    { id: "episodes", label: "browse.episodes" },
  ];

  let results = $derived(browse.results);
  let counts = $derived({
    tracks: results?.tracks.length ?? 0,
    albums: results?.albums.length ?? 0,
    artists: results?.artists.length ?? 0,
    playlists: results?.playlists.length ?? 0,
    shows: results?.shows.length ?? 0,
    episodes: results?.episodes.length ?? 0,
  });
  let activeHits = $derived<SearchHit[]>(results?.[browse.searchTab] ?? []);

  function meta(hit: SearchHit): string {
    const artists = (hit.artists ?? []).map((artist) => artist.name).join(", ");
    switch (browse.searchTab) {
      case "albums":
        return artists || t("browse.album");
      case "artists":
        return t("browse.artist");
      case "playlists":
        return `${hit.author || t("browse.playlist")}${
          hit.followersCount > 0 ? ` · ${t("browse.followers", { count: hit.followersCount })}` : ""
        }`;
      case "shows":
        return t("browse.podcast");
      case "episodes":
        return [hit.album?.name, artists, hit.duration > 0 ? formatTime(hit.duration / 1000) : ""]
          .filter(Boolean)
          .join(" · ");
      default:
        return [artists, hit.album?.name, hit.duration > 0 ? formatTime(hit.duration / 1000) : ""]
          .filter(Boolean)
          .join(" · ");
    }
  }
</script>

<section class="browse-view" aria-labelledby="search-view-title">
  <header class="view-head">
    <div>
      <p class="eyebrow">{t("browse.search")}</p>
      <h1 id="search-view-title" class="view-title">{t("browse.searchTitle")}</h1>
    </div>
    {#if browse.query}
      <span class="view-subtitle">{browse.query}</span>
    {/if}
  </header>

  {#if browse.searchError}
    <p class="browse-banner is-error">{browse.searchError}</p>
  {/if}

  {#if browse.searching && !results}
    <div class="skeleton-wrap">
      <Skeleton rows={8} height={16} />
    </div>
  {:else if results}
    <div class="tab-bar" role="group" aria-label={t("browse.search")}>
      {#each TABS as tab (tab.id)}
        <button
          class="tab"
          class:is-active={browse.searchTab === tab.id}
          type="button"
          aria-pressed={browse.searchTab === tab.id}
          onclick={() => browse.setSearchTab(tab.id)}
        >
          {t(tab.label)}
          <span class="tab-count">{counts[tab.id]}</span>
        </button>
      {/each}
    </div>

    {#if activeHits.length === 0}
      <p class="empty-panel">{t("browse.searchTabEmpty")}</p>
    {:else}
      <ul class="result-list">
        {#each activeHits as hit (hit.uri)}
          <li class="result-row" class:is-current={hit.uri === player.state.track.uri}>
            <button
              class="result-main"
              type="button"
              onclick={() => void browse.playSearchHit(hit, browse.searchTab)}
            >
              <span class="result-cover" class:is-round={browse.searchTab === "artists"}>
                {#if hit.image}
                  <img
                    src={coverURL(hit.image, 40)}
                    alt=""
                    width="40"
                    height="40"
                    loading="eager"
                    decoding="async"
                  />
                {:else}
                  <Icon name="note" size={16} />
                {/if}
              </span>
              <span class="result-copy">
                <span class="result-title">{hit.name}</span>
                <span class="result-meta">{meta(hit)}</span>
              </span>
            </button>

            {#if browse.searchTab === "tracks"}
              <div class="row-actions">
                <button
                  class="icon-button"
                  type="button"
                  aria-label={t("browse.playNamed", { title: hit.name })}
                  onclick={() => void browse.playSearchHit(hit, "tracks")}
                >
                  <Icon name="play" size={13} filled />
                </button>
                <button
                  class="icon-button"
                  type="button"
                  aria-label={t("browse.queueNamed", { title: hit.name })}
                  onclick={() => void browse.addToQueue(hit.uri)}
                >
                  <Icon name="plus" size={13} />
                </button>
                <button
                  class="icon-button"
                  type="button"
                  aria-label={t("browse.radioNamed", { title: hit.name })}
                  onclick={() => void browse.startRadio(hit.uri)}
                >
                  <Icon name="radio" size={13} />
                </button>
              </div>
            {:else if browse.searchTab === "episodes"}
              <div class="row-actions">
                <button
                  class="icon-button"
                  type="button"
                  aria-label={t("browse.playNamed", { title: hit.name })}
                  onclick={() => void browse.playSearchHit(hit, "episodes")}
                >
                  <Icon name="play" size={13} filled />
                </button>
              </div>
            {:else}
              <div class="row-actions">
                <span class="row-arrow">
                  <Icon name="external" size={13} />
                </span>
              </div>
            {/if}
          </li>
        {/each}
      </ul>
    {/if}
  {:else}
    <div class="empty-panel">
      <Icon name="search" size={22} />
      <h2>{t("browse.searchPrompt")}</h2>
      <p>{t("browse.searchBody")}</p>
    </div>
  {/if}
</section>
