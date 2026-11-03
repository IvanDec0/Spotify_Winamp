import type { Component } from "svelte";
import AlbumArt from "./components/AlbumArt.svelte";
import DevicePicker from "./components/DevicePicker.svelte";
import Progress from "./components/Progress.svelte";
import Queue from "./components/Queue.svelte";
import Repeat from "./components/Repeat.svelte";
import SettingsButton from "./components/SettingsButton.svelte";
import Shuffle from "./components/Shuffle.svelte";
import TitleBar from "./components/TitleBar.svelte";
import TrackInfo from "./components/TrackInfo.svelte";
import Transport from "./components/Transport.svelte";
import Visualizer from "./components/Visualizer.svelte";
import Volume from "./components/Volume.svelte";

// The registry is the complete rendering surface offered to declarative skins.
// Unknown names never reach this map because the Go validator rejects them.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export const componentRegistry: Record<string, Component<any>> = {
  titlebar: TitleBar,
  "album-art": AlbumArt,
  "track-info": TrackInfo,
  progress: Progress,
  "transport-controls": Transport,
  volume: Volume,
  shuffle: Shuffle,
  repeat: Repeat,
  queue: Queue,
  visualizer: Visualizer,
  "device-picker": DevicePicker,
  "settings-button": SettingsButton,
};
