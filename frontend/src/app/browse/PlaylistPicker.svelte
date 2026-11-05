<script lang="ts">
  import { coverURL } from "../../lib/artwork";
  import { t } from "../../shared/i18n";
  import Dialog from "../../shared/ui/Dialog.svelte";
  import Skeleton from "../../shared/ui/Skeleton.svelte";
  import Icon from "./Icon.svelte";
  import { browse } from "./browse.svelte";

  interface Props {
    trackUri: string | null;
    onclose: () => void;
  }

  let { trackUri, onclose }: Props = $props();

  $effect(() => {
    if (trackUri) void browse.loadEditablePlaylists();
  });

  async function choose(playlistUri: string): Promise<void> {
    if (!trackUri) return;
    await browse.addToPlaylist(trackUri, playlistUri);
    onclose();
  }
</script>

<Dialog open={trackUri !== null} title={t("browse.playlistPickerTitle")} {onclose}>
  {#if browse.playlistsLoading && browse.editablePlaylists.length === 0}
    <Skeleton rows={4} height={16} />
  {:else if browse.editablePlaylists.length === 0}
    <p class="empty-panel">{t("browse.noEditablePlaylists")}</p>
  {:else}
    <ul class="picker-list">
      {#each browse.editablePlaylists as playlist (playlist.uri)}
        <li>
          <button class="picker-row" type="button" onclick={() => void choose(playlist.uri)}>
            <span class="result-cover is-small" aria-hidden="true">
              {#if playlist.imageUrl}
                <img
                  src={coverURL(playlist.imageUrl, 32)}
                  alt=""
                  width="32"
                  height="32"
                  loading="lazy"
                  decoding="async"
                />
              {:else}
                <Icon name="library" size={14} />
              {/if}
            </span>
            <span class="result-copy">
              <span class="result-title">{playlist.name}</span>
              <span class="result-meta">
                {playlist.owner || t("browse.playlist")} · {t("browse.tracksCount", {
                  count: playlist.length,
                })}
              </span>
            </span>
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</Dialog>
