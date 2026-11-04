import { Service } from "../../../bindings/cadence/internal/player";
import { call } from "./client";

/**
 * Typed player facade. Every generated call goes through `call` so failures
 * surface as ApiError instances.
 */
export const player = {
  getState: () => call(() => Service.GetState()),
  getDevices: () => call(() => Service.GetDevices()),
  getQueue: () => call(() => Service.GetQueue()),
  play: () => call(() => Service.Play()),
  pause: () => call(() => Service.Pause()),
  togglePlay: () => call(() => Service.TogglePlay()),
  next: () => call(() => Service.Next()),
  previous: () => call(() => Service.Previous()),
  seek: (positionMs: number) => call(() => Service.Seek(positionMs)),
  setVolume: (volume: number) => call(() => Service.SetVolume(volume)),
  setShuffle: (enabled: boolean) => call(() => Service.SetShuffle(enabled)),
  setRepeat: (mode: string) => call(() => Service.SetRepeat(mode)),
  transfer: (deviceId: string) => call(() => Service.Transfer(deviceId)),
};

export type PlayerApi = typeof player;
