<script lang="ts">
  import { Status } from "../../../../bindings/cadence/internal/player/models";
  import { player } from "../../player.svelte";

  interface Props {
    showAlbum?: boolean;
    showStatus?: boolean;
    align?: "left" | "center";
  }

  let { showAlbum = true, showStatus = true, align = "left" }: Props = $props();

  let playing = $derived(player.state.status === Status.StatusPlaying);
  let stateError = $derived(player.state.error?.message ?? "");
  let artists = $derived(
    player.state.track.artists?.join(", ") ||
      (player.state.error?.code === "no_active_device"
        ? "Choose a Spotify device"
        : "Unknown artist"),
  );
  let displayTitle = $derived(
    player.state.track.title ||
      (player.state.error?.code === "no_active_device" ? "No active device" : "Nothing playing"),
  );
  let statusLabel = $derived(
    player.state.status === Status.StatusPlaying
      ? "Now playing"
      : player.state.status === Status.StatusPaused
        ? "Paused"
        : "Stopped",
  );
</script>

<div class="cad-track-info" class:is-centered={align === "center"} data-component="track-info">
  {#if showStatus}
    <div class="cad-track-status">
      <span
        class="cad-status-dot"
        class:is-playing={playing}
        class:is-error={Boolean(player.state.error)}
        aria-hidden="true"
      ></span>
      <span class:is-error={Boolean(player.state.error)} title={stateError || statusLabel}>
        {stateError || statusLabel}
      </span>
      {#if player.state.shuffle}<span class="cad-mode-tag">SHUF</span>{/if}
    </div>
  {/if}

  <h1 class="cad-track-title" title={displayTitle}>{displayTitle}</h1>
  <p class="cad-track-artists" title={artists}>{artists}</p>
  {#if showAlbum && player.state.track.album}
    <p class="cad-track-album" title={player.state.track.album}>{player.state.track.album}</p>
  {/if}
</div>
