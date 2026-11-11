import { Service } from "../../../bindings/cadence/internal/wailsapi/audio";
import type { EngineSettings, PlaybackSettings } from "./types";
import { call } from "./client";

/**
 * Typed audio engine facade. Both `getState` functions live on their own
 * namespace so player and engine imports never collide.
 */
export const audio = {
  getState: () => call(() => Service.GetState()),
  getEngineSettings: () => call(() => Service.GetEngineSettings()),
  setEngineSettings: (settings: EngineSettings) => call(() => Service.SetEngineSettings(settings)),
  getPlaybackSettings: () => call(() => Service.GetPlaybackSettings()),
  setPlaybackSettings: (settings: PlaybackSettings) =>
    call(() => Service.SetPlaybackSettings(settings)),
  installSoloist: () => call(() => Service.InstallSoloist()),
  saveSoloistAPIKey: (key: string) => call(() => Service.SaveSoloistAPIKey(key)),
  clearSoloistAPIKey: () => call(() => Service.ClearSoloistAPIKey()),
  startSoloist: () => call(() => Service.StartSoloist()),
  stopSoloist: () => call(() => Service.StopSoloist()),
  restartSoloist: () => call(() => Service.RestartSoloist()),
  openSoloistDashboard: () => call(() => Service.OpenSoloistDashboard()),
  openSoloistDownloads: () => call(() => Service.OpenSoloistDownloads()),
  openPairing: () => call(() => Service.OpenPairing()),
  resetSession: () => call(() => Service.ResetSession()),
  restart: () => call(() => Service.Restart()),
  getWebPlayerStatus: () => call(() => Service.GetWebPlayerStatus()),
  connectWebPlayer: () => call(() => Service.ConnectWebPlayer()),
  disconnectWebPlayer: () => call(() => Service.DisconnectWebPlayer()),
  installWebPlayerRuntime: () => call(() => Service.InstallWebPlayerRuntime()),
  startWebPlayer: () => call(() => Service.StartWebPlayer()),
  stopWebPlayer: () => call(() => Service.StopWebPlayer()),
  configureWebPlayerApp: () => call(() => Service.ConfigureWebPlayerApp()),
  generateSoloistAPIKey: (rotate: boolean) => call(() => Service.GenerateSoloistAPIKey(rotate)),
  playlists: (offset: number, limit: number) => call(() => Service.Playlists(offset, limit)),
  albums: (offset: number, limit: number) => call(() => Service.Albums(offset, limit)),
  artists: (offset: number, limit: number) => call(() => Service.Artists(offset, limit)),
  contextTracks: (uri: string) => call(() => Service.ContextTracks(uri)),
  yourEpisodes: () => call(() => Service.YourEpisodes()),
  search: (query: string, limit: number) => call(() => Service.Search(query, limit)),
  startRadio: (uri: string) => call(() => Service.StartRadio(uri)),
  playContext: (uri: string, skipToURI: string) => call(() => Service.PlayContext(uri, skipToURI)),
  playTrack: (uri: string) => call(() => Service.PlayTrack(uri)),
  addToQueue: (uri: string) => call(() => Service.AddToQueue(uri)),
  setLiked: (uri: string, liked: boolean) => call(() => Service.SetLiked(uri, liked)),
  isLiked: (uri: string) => call(() => Service.IsLiked(uri)),
  saveAlbum: (uri: string, saved: boolean) => call(() => Service.SaveAlbum(uri, saved)),
  isAlbumSaved: (uri: string) => call(() => Service.IsAlbumSaved(uri)),
  saveShow: (uri: string, saved: boolean) => call(() => Service.SaveShow(uri, saved)),
  isShowSaved: (uri: string) => call(() => Service.IsShowSaved(uri)),
  followArtist: (uri: string, following: boolean) =>
    call(() => Service.FollowArtist(uri, following)),
  isArtistFollowed: (uri: string) => call(() => Service.IsArtistFollowed(uri)),
  addToPlaylist: (playlistURI: string, uri: string) =>
    call(() => Service.AddToPlaylist(playlistURI, uri)),
  removeFromPlaylist: (playlistURI: string, uri: string) =>
    call(() => Service.RemoveFromPlaylist(playlistURI, uri)),
};

export type AudioApi = typeof audio;
