import { Service } from "../../../bindings/cadence/internal/golibrespot";
import { call } from "./client";

/**
 * Typed audio engine facade. Both `getState` functions live on their own
 * namespace so player and engine imports never collide.
 */
export const audio = {
  getState: () => call(() => Service.GetState()),
  openPairing: () => call(() => Service.OpenPairing()),
  resetSession: () => call(() => Service.ResetSession()),
  restart: () => call(() => Service.Restart()),
  playlists: (offset: number, limit: number) => call(() => Service.Playlists(offset, limit)),
  albums: (offset: number, limit: number) => call(() => Service.Albums(offset, limit)),
  artists: (offset: number, limit: number) => call(() => Service.Artists(offset, limit)),
  contextTracks: (uri: string) => call(() => Service.ContextTracks(uri)),
  search: (query: string, limit: number) => call(() => Service.Search(query, limit)),
  startRadio: (uri: string) => call(() => Service.StartRadio(uri)),
  playContext: (uri: string, skipToURI: string) => call(() => Service.PlayContext(uri, skipToURI)),
  playTrack: (uri: string) => call(() => Service.PlayTrack(uri)),
  addToQueue: (uri: string) => call(() => Service.AddToQueue(uri)),
};

export type AudioApi = typeof audio;
