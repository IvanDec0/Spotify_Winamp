import { mount } from "svelte";
import "./browse.css";
import BrowseApp from "./BrowseApp.svelte";
import { player } from "../../lib/player.svelte";
import { skins } from "../../lib/skins.svelte";

let mounted = false;

/**
 * Entry point for the dedicated Browse surface (`/?surface=browse`). The
 * compact player store and skin catalog are shared code; the shell itself is
 * app-owned and only the theme tokens and optional browse.css come from skins.
 */
export function mountBrowse(): void {
  if (mounted) return;
  mounted = true;
  void player.start();
  void skins.start();
  mount(BrowseApp, { target: document.getElementById("app")! });
}
