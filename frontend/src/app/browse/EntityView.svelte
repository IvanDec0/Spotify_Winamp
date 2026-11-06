<script lang="ts">
  import { tick } from "svelte";
  import { coverURL } from "../../lib/artwork";
  import { formatTime } from "../../lib/format";
  import { music } from "../../lib/music.svelte";
  import { player } from "../../lib/player.svelte";
  import { t } from "../../shared/i18n";
  import Button from "../../shared/ui/Button.svelte";
  import Skeleton from "../../shared/ui/Skeleton.svelte";
  import VirtualList from "../../shared/ui/VirtualList.svelte";
  import Icon from "./Icon.svelte";
  import PlaylistPicker from "./PlaylistPicker.svelte";
  import { browse } from "./browse.svelte";

  let pickerTrack = $state<string | null>(null);
  let backButton = $state<HTMLButtonElement>();

  let entity = $derived(browse.entity);
  let context = $derived(browse.context);
  let items = $derived(context?.items ?? []);
  let coverSrc = $derived(entity?.image ? coverURL(entity.image, 120) : "");
  let canToggle = $derived(
    entity?.kind === "album" || entity?.kind === "show" || entity?.kind === "artist",
  );
  let canRadio = $derived(
    !music.soloistMode &&
      (entity?.kind === "album" || entity?.kind === "artist" || entity?.kind === "playlist"),
  );
  let editable = $derived(entity?.kind === "playlist" && Boolean(entity.canEdit));
  let kindLabel = $derived.by(() => {
    switch (entity?.kind) {
      case "album":
        return t("browse.album");
      case "artist":
        return t("browse.artist");
      case "show":
        return t("browse.show");
      default:
        return t("browse.playlist");
    }
  });
  let toggleLabel = $derived.by(() => {
    if (browse.entitySaved) return t("browse.remove");
    return entity?.kind === "artist" ? t("browse.follow") : t("browse.save");
  });
  let loadingStatus = $derived(
    t("browse.loadingTracks", {
      cached: context?.cached ?? 0,
      length: context?.length ?? 0,
    }),
  );

  $effect(() => {
    const uri = browse.entity?.uri;
    if (!uri) return;
    void tick().then(() => backButton?.focus());
  });

  function meta(artists: string[] | null, album: string): string {
    return [(artists ?? []).join(", "), album].filter(Boolean).join(" · ");
  }

  function goBack(): void {
    browse.back();
    void tick().then(() => {
      document.querySelector<HTMLElement>("main .view-title")?.focus();
    });
  }
</script>

{#if entity}
  <section class="entity-view" aria-labelledby="entity-title" aria-busy={browse.contextLoading}>
    <button class="back-button" type="button" bind:this={backButton} onclick={goBack}>
      <Icon name="back" size={14} />
      {t("browse.back")}
    </button>

    <header class="entity-head">
      <div class="entity-cover" aria-hidden="true">
        {#if coverSrc}
          <img src={coverSrc} alt="" width="104" height="104" loading="eager" decoding="async" />
        {:else}
          <Icon name="note" size={30} />
        {/if}
      </div>
      <div class="entity-copy">
        <span class="eyebrow">{kindLabel}</span>
        <h1 id="entity-title" class="entity-title">{entity.title}</h1>
        <p class="entity-meta">
          {entity.subtitle || entity.owner || ""}
        </p>
        <div class="entity-actions">
          <Button
            variant="primary"
            size="sm"
            disabled={browse.busy || items.length === 0}
            onclick={() => void browse.playEntity()}
          >
            <Icon name="play" size={13} filled />
            {t("browse.playAll")}
          </Button>
          {#if canToggle}
            <Button
              variant="secondary"
              size="sm"
              disabled={browse.busy}
              aria-pressed={browse.entitySaved}
              onclick={() => void browse.toggleSave()}
            >
              {#if browse.entitySaved}<Icon name="check" size={13} />{/if}
              {toggleLabel}
            </Button>
          {/if}
          {#if canRadio}
            <Button
              variant="ghost"
              size="sm"
              disabled={browse.busy}
              onclick={() => void browse.startRadio(entity.uri)}
            >
              <Icon name="radio" size={13} />
              {t("browse.radio")}
            </Button>
          {/if}
        </div>
      </div>
    </header>

    {#if browse.contextError}
      <div class="browse-banner is-error" role="alert" aria-live="assertive">
        <span>{browse.contextError}</span>
        <Button variant="ghost" size="sm" onclick={() => void browse.reloadEntity()}>
          {t("ui.retry")}
        </Button>
      </div>
    {/if}

    {#if browse.contextLoading && !context}
      <div class="skeleton-wrap" aria-busy="true" aria-label={t("browse.loading")}>
        <Skeleton rows={8} height={16} />
      </div>
    {:else if context && !context.ready}
      <p class="entity-status">{loadingStatus}</p>
    {:else if !context && !browse.contextError}
      <p class="empty-panel">{t("browse.emptyContext")}</p>
    {/if}

    {#if context && context.ready && items.length === 0}
      <p class="empty-panel">{t("browse.emptyContext")}</p>
    {:else if items.length > 0}
      <div class="entity-list">
        <VirtualList {items} itemHeight={44} key={(item, index) => `${item.uri}:${index}`}>
          {#snippet children(item, index)}
            <div class="entity-row" class:is-current={item.uri === player.state.track.uri}>
              <span class="entity-index" aria-hidden="true">
                {#if item.uri === player.state.track.uri}
                  <Icon name="play" size={10} filled />
                {:else}
                  {String(index + 1).padStart(2, "0")}
                {/if}
              </span>
              <span class="entity-cover is-small" aria-hidden="true">
                {#if item.cover}
                  <img
                    src={coverURL(item.cover, 32)}
                    alt=""
                    width="32"
                    height="32"
                    loading="lazy"
                    decoding="async"
                  />
                {:else}
                  <Icon name="note" size={13} />
                {/if}
              </span>
              <button
                class="entity-copy is-row"
                type="button"
                onclick={() => void browse.playItem(item)}
              >
                <span class="entity-title is-row">{item.title || item.uri}</span>
                <span class="entity-meta is-row">
                  {meta(item.artists, item.album)}
                </span>
              </button>
              <span class="entity-time">
                {item.durationMs > 0 ? formatTime(item.durationMs / 1000) : ""}
              </span>
              <div class="row-actions">
                <button
                  class="icon-button"
                  type="button"
                  title={t("browse.playNamed", { title: item.title || item.uri })}
                  aria-label={t("browse.playNamed", { title: item.title || item.uri })}
                  onclick={() => void browse.playItem(item)}
                >
                  <Icon name="play" size={12} filled />
                </button>
                <button
                  class="icon-button"
                  type="button"
                  title={t("browse.queueNamed", { title: item.title || item.uri })}
                  aria-label={t("browse.queueNamed", { title: item.title || item.uri })}
                  onclick={() => void browse.addToQueue(item.uri)}
                >
                  <Icon name="plus" size={12} />
                </button>
                <button
                  class="icon-button"
                  type="button"
                  title={t("browse.playlistNamed", { title: item.title || item.uri })}
                  aria-label={t("browse.playlistNamed", { title: item.title || item.uri })}
                  onclick={() => {
                    pickerTrack = item.uri;
                  }}
                >
                  <Icon name="library" size={12} />
                </button>
                {#if editable}
                  <button
                    class="icon-button is-danger"
                    type="button"
                    title={t("browse.removeNamed", { title: item.title || item.uri })}
                    aria-label={t("browse.removeNamed", { title: item.title || item.uri })}
                    onclick={() => void browse.removeFromPlaylist(item.uri)}
                  >
                    <Icon name="trash" size={12} />
                  </button>
                {/if}
              </div>
            </div>
          {/snippet}
        </VirtualList>
      </div>
    {/if}
  </section>

  <PlaylistPicker trackUri={pickerTrack} onclose={() => (pickerTrack = null)} />
{/if}
