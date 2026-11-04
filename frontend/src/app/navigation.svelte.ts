export const PANELS = ["none", "library", "queue", "settings", "skins", "devices"] as const;
export type Panel = (typeof PANELS)[number];
export type OpenPanel = Exclude<Panel, "none">;

export type Route =
  { name: "home" } | { name: "search"; query: string } | { name: "entity"; uri: string };

const HOME: Route = { name: "home" };

/**
 * One navigation store owns which side panel is open and the browse back
 * stack. Only one panel can be open at a time; the home route is the root.
 */
export class Navigation {
  panel = $state<Panel>("none");
  stack = $state<Route[]>([HOME]);

  get route(): Route {
    return this.stack[this.stack.length - 1] ?? HOME;
  }

  open(panel: OpenPanel): void {
    this.panel = panel;
  }

  close(): void {
    this.panel = "none";
  }

  toggle(panel: OpenPanel): void {
    this.panel = this.panel === panel ? "none" : panel;
  }

  push(route: Route): void {
    this.stack = [...this.stack, route];
  }

  back(): void {
    if (this.stack.length <= 1) return;
    this.stack = this.stack.slice(0, -1);
  }

  reset(): void {
    this.panel = "none";
    this.stack = [HOME];
  }
}

export const navigation = new Navigation();
