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
  let artists = $derived(player.state.track.artists?.join(", ") || "Unknown artist");
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
      <span class="cad-status-dot" class:is-playing={playing} aria-hidden="true"></span>
      <span>{statusLabel}</span>
      {#if player.state.shuffle}<span class="cad-mode-tag">SHUF</span>{/if}
    </div>
  {/if}

  <h1 class="cad-track-title" title={player.state.track.title}>{player.state.track.title}</h1>
  <p class="cad-track-artists" title={artists}>{artists}</p>
  {#if showAlbum && player.state.track.album}
    <p class="cad-track-album" title={player.state.track.album}>{player.state.track.album}</p>
  {/if}
</div>
