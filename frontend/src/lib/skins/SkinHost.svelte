<script lang="ts">
  import { mount, onMount, unmount } from "svelte";
  import { system } from "../../shared/api";
  import type { LayoutNode, SkinRuntime } from "../../shared/api/types";
  import { preferences } from "../preferences.svelte";
  import { skins } from "../skins.svelte";
  import RuntimeRoot from "./RuntimeRoot.svelte";
  import { componentRegistry } from "./registry";
  import { titlebarDragTarget } from "./drag";
  import baseCSS from "./skin-base.css?raw";

  interface Props {
    runtime: SkinRuntime;
    theme: string;
    queueOpen: boolean;
  }

  let { runtime, theme, queueOpen }: Props = $props();

  let host: HTMLDivElement;
  let dragCandidate = false;
  let dragButtonDown = false;
  let dragStartX = 0;
  let dragStartY = 0;
  let colorTimer: ReturnType<typeof setTimeout> | null = null;

  function layoutIssue(
    node: LayoutNode | undefined,
    depth = 1,
    count = { value: 0 },
  ): string | null {
    if (!node) return "layout is empty";
    count.value += 1;
    if (count.value > 500) return "layout exceeds the node limit";
    if (depth > 12) return "layout exceeds the depth limit";
    if (node.type === "component") {
      if (!node.component || !componentRegistry[node.component]) {
        return `unsupported component ${node.component ?? "(missing)"}`;
      }
      return null;
    }
    if (
      node.type === "box" ||
      node.type === "text" ||
      node.type === "image" ||
      node.type === "button"
    ) {
      return null;
    }
    if (
      node.type !== "stack" &&
      node.type !== "row" &&
      node.type !== "column" &&
      node.type !== "grid"
    ) {
      return `unsupported container ${node.type}`;
    }
    for (const child of node.children ?? []) {
      const issue = layoutIssue(child, depth + 1, count);
      if (issue) return issue;
    }
    return null;
  }

  function syncColors(): void {
    if (!host) return;
    const style = getComputedStyle(host);
    const names = [
      "--bg",
      "--surface",
      "--surface-2",
      "--surface-3",
      "--text",
      "--muted",
      "--faint",
      "--line",
      "--line-strong",
      "--panel",
      "--accent",
      "--accent-2",
      "--danger",
    ];
    const palette: Record<string, string> = {};
    for (const name of names) {
      const value = style.getPropertyValue(name).trim();
      if (value && !/[;{}]/.test(value)) {
        palette[name] = value;
      }
    }
    skins.setPalette(palette);
  }

  function scheduleColorSync(): void {
    if (colorTimer) clearTimeout(colorTimer);
    colorTimer = setTimeout(() => {
      colorTimer = null;
      syncColors();
    }, 40);
  }

  $effect(() => {
    // Remount when the skin or theme changes. Queue changes are applied as an
    // attribute so opening the queue does not reset player interaction state.
    const activeRuntime = runtime;
    const activeTheme = theme;
    if (!host || activeRuntime.builtin) return;

    const issue = layoutIssue(activeRuntime.layout.root);
    if (issue) {
      void skins.reportRuntimeFailure(activeRuntime.id, issue);
      return;
    }

    const shadow = host.shadowRoot ?? host.attachShadow({ mode: "open" });
    shadow.replaceChildren();

    const baseStyle = document.createElement("style");
    baseStyle.textContent = baseCSS;
    shadow.appendChild(baseStyle);

    const stylesheetFailure = (): void => {
      void skins.reportRuntimeFailure(activeRuntime.id, "the skin stylesheet could not be loaded");
    };

    const baseLink = document.createElement("link");
    baseLink.rel = "stylesheet";
    baseLink.href = activeRuntime.stylesheetUrl;
    baseLink.addEventListener("load", syncColors);
    baseLink.addEventListener("error", stylesheetFailure);
    shadow.appendChild(baseLink);

    let themeLink: HTMLLinkElement | null = null;
    const themeURL = activeRuntime.themeStylesheets?.[activeTheme];
    if (themeURL) {
      themeLink = document.createElement("link");
      themeLink.rel = "stylesheet";
      themeLink.href = themeURL;
      themeLink.addEventListener("load", syncColors);
      themeLink.addEventListener("error", stylesheetFailure);
      shadow.appendChild(themeLink);
    }

    let app: Record<string, unknown> | null = null;
    try {
      app = mount(RuntimeRoot, {
        target: shadow,
        props: { runtime: activeRuntime },
      }) as Record<string, unknown>;
      skins.reportRuntimeReady(activeRuntime.id, activeTheme);
    } catch (error) {
      void skins.reportRuntimeFailure(
        activeRuntime.id,
        error instanceof Error ? error.message : "the skin runtime could not be mounted",
      );
    }

    host.dataset.theme = activeTheme;
    scheduleColorSync();

    return () => {
      if (colorTimer) {
        clearTimeout(colorTimer);
        colorTimer = null;
      }
      baseLink.removeEventListener("load", syncColors);
      baseLink.removeEventListener("error", stylesheetFailure);
      if (themeLink) {
        themeLink.removeEventListener("load", syncColors);
        themeLink.removeEventListener("error", stylesheetFailure);
      }
      if (app) unmount(app);
      shadow.replaceChildren();
    };
  });

  $effect(() => {
    if (!host) return;
    host.dataset.theme = theme;
    host.dataset.queueOpen = queueOpen ? "true" : "false";
    host.dataset.reducedMotion = preferences.reducedMotion ? "true" : "false";
    scheduleColorSync();
  });

  function handleMouseDown(event: MouseEvent): void {
    if (event.button !== 0 || event.detail !== 1) return;
    const titlebar = titlebarDragTarget(event.composedPath());
    if (!titlebar) return;
    dragCandidate = true;
    dragButtonDown = true;
    dragStartX = event.clientX;
    dragStartY = event.clientY;
  }

  function handleMouseMove(event: MouseEvent): void {
    if (!dragCandidate || !dragButtonDown) return;
    if (Math.hypot(event.clientX - dragStartX, event.clientY - dragStartY) < 3) return;
    dragCandidate = false;
    // Wails' drag detection cannot see styles inside a Shadow DOM, so the
    // composed titlebar path drives the same native drag command explicitly.
    system.invoke("wails:drag");
  }

  function handleMouseUp(): void {
    dragCandidate = false;
    dragButtonDown = false;
  }

  function handleBlur(): void {
    dragCandidate = false;
    dragButtonDown = false;
  }

  onMount(() => {
    window.addEventListener("mousedown", handleMouseDown, true);
    window.addEventListener("mousemove", handleMouseMove, true);
    window.addEventListener("mouseup", handleMouseUp, true);
    window.addEventListener("blur", handleBlur);
    return () => {
      window.removeEventListener("mousedown", handleMouseDown, true);
      window.removeEventListener("mousemove", handleMouseMove, true);
      window.removeEventListener("mouseup", handleMouseUp, true);
      window.removeEventListener("blur", handleBlur);
    };
  });
</script>

<div bind:this={host} class="skin-host" aria-label={`${runtime.name} skin`}></div>
