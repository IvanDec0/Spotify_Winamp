<script lang="ts">
  import { Status } from "../../../shared/api/types";
  import { t } from "../../../shared/i18n";
  import { playerErrorCopy } from "../../player-error";
  import { player } from "../../player.svelte";

  interface Props {
    showAlbum?: boolean;
    showStatus?: boolean;
    align?: "left" | "center";
  }

  let { showAlbum = true, showStatus = true, align = "left" }: Props = $props();

  let playing = $derived(player.state.status === Status.StatusPlaying);
  let errorCopy = $derived(playerErrorCopy(player.state.error));
  let statusLabel = $derived(
    player.state.status === Status.StatusPlaying
      ? t("browse.nowPlaying")
      : player.state.status === Status.StatusPaused
        ? t("player.paused")
        : t("player.stopped"),
  );
  let statusText = $derived(errorCopy.statusKey ? t(errorCopy.statusKey) : statusLabel);
  let statusDetail = $derived(errorCopy.detail || statusText);
  let artists = $derived(
    player.state.track.artists?.join(", ") ||
      (errorCopy.hintKey ? t(errorCopy.hintKey) : t("player.unknownArtist")),
  );
  let displayTitle = $derived(
    player.state.track.title ||
      (errorCopy.titleKey ? t(errorCopy.titleKey) : t("browse.nothingPlaying")),
  );
</script>

<div
  class="cad-track-info"
  class:is-centered={align === "center"}
  class:is-playing={playing}
  data-component="track-info"
>
  {#if showStatus}
    <div class="cad-track-status">
      <span
        class="cad-status-dot"
        class:is-playing={playing}
        class:is-error={Boolean(player.state.error)}
        aria-hidden="true"
      ></span>
      <span class:is-error={Boolean(player.state.error)} title={statusDetail}>
        {statusText}
      </span>
      {#if player.state.shuffle}<span class="cad-mode-tag">{t("player.shuffleShort")}</span>{/if}
    </div>
  {/if}

  {#key player.state.track.id}
    <div class="cad-track-copy">
      <h1 class="cad-track-title" title={displayTitle}>{displayTitle}</h1>
      <p class="cad-track-artists" title={artists}>{artists}</p>
      {#if showAlbum && player.state.track.album}
        <p class="cad-track-album" title={player.state.track.album}>{player.state.track.album}</p>
      {/if}
    </div>
  {/key}
</div>
