import { mount } from "svelte";
import App from "./App.svelte";
import { initI18n } from "./shared/i18n";
import "./app.css";

await initI18n();

// The dedicated Browse window shares the runtime and i18n but loads its own
// entry chunk. The compact window never downloads the Browse shell.
if (new URLSearchParams(location.search).get("surface") === "browse") {
  const { mountBrowse } = await import("./app/browse");
  mountBrowse();
} else {
  mount(App, { target: document.getElementById("app")! });
}
