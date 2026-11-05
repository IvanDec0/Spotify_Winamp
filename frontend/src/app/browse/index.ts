import { mount } from "svelte";
import "./browse.css";
import BrowseApp from "./BrowseApp.svelte";
import { music } from "../../lib/music.svelte";
import { player } from "../../lib/player.svelte";
import { preferences } from "../../lib/preferences.svelte";
import { skins } from "../../lib/skins.svelte";
import { browse, normalizeSection } from "./browse.svelte";

let mounted = false;

/**
 * Entry point for the dedicated Browse surface (`/?surface=browse`). The
 * compact player and audio stores and the skin catalog are shared code; the
 * shell itself is app-owned and only the theme tokens and optional browse.css
 * come from skins.
 */
export function mountBrowse(): void {
  if (mounted) return;
  mounted = true;
  const section = normalizeSection(new URLSearchParams(location.search).get("section"), "library");
  browse.setInitialSection(section);
  preferences.startStorageSync();
  void player.start();
  void music.start();
  void skins.start();
  mount(BrowseApp, { target: document.getElementById("app")! });
}
