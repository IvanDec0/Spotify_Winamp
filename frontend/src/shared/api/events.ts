import { Events } from "@wailsio/runtime";
import type { AudioState, PlayerState, SkinCatalog } from "./types";

type Handler<T> = (payload: T) => void;

/**
 * Subscribes to player snapshots. The generated binding only transports the
 * Wails envelope; callers receive the payload.
 */
export function onPlayerState(handler: Handler<PlayerState>): () => void {
  return Events.On("player:state", (event) => handler(event.data as PlayerState));
}

/**
 * Subscribes to audio engine state changes.
 */
export function onAudioState(handler: Handler<AudioState>): () => void {
  return Events.On("audio:state", (event) => handler(event.data as AudioState));
}

/**
 * Subscribes to skin catalog changes.
 */
export function onSkinsChanged(handler: Handler<SkinCatalog>): () => void {
  return Events.On("skins:changed", (event) => handler(event.data as SkinCatalog));
}
