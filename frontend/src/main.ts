import { mount } from "svelte";
import App from "./App.svelte";
import { initI18n } from "./shared/i18n";
import "./app.css";

await initI18n();

mount(App, { target: document.getElementById("app")! });
