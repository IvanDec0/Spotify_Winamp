export { RepeatMode, Status } from "../../../bindings/cadence/internal/playback/models";

export type {
  Device,
  PlayerError,
  PlayerState,
  QueueItem,
  Track,
} from "../../../bindings/cadence/internal/playback/models";

export type {
  ContextTrackInfo,
  LibraryContext,
} from "../../../bindings/cadence/internal/catalog/models";

export type {
  AlbumPage,
  ArtistPage,
  AudioState,
  LibraryAlbum,
  LibraryArtist,
  LibraryPlaylist,
  Pairing,
  PlaylistPage,
  SearchAlbum,
  SearchArtist,
  SearchGroup,
  SearchHit,
  SearchResults,
  SoloistState,
} from "../../../bindings/cadence/internal/engine/models";

export type {
  EngineSettings,
  PlaybackSettings,
  WebPlayerStatus,
} from "../../../bindings/cadence/internal/wailsapi/audio/models";

export type {
  InstallResult,
  Layout,
  LayoutNode,
  SkinCatalog,
  SkinRuntime,
  SkinSummary,
} from "../../../bindings/cadence/internal/skins/models";

import type { Frame } from "../../../bindings/cadence/internal/visualizer/models";

/**
 * Analyzed monitor frame. Go always sends arrays; the generated model marks
 * them nullable, so the frontend narrows them once at the event boundary.
 */
export type VisualizerFrame = Omit<Frame, "bands" | "waveform"> & {
  bands: number[];
  waveform: number[];
};
