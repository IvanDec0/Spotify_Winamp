import { beforeEach, describe, expect, it, vi } from "vitest";

type EventHandler = (event: { data: unknown }) => void;
type OnHandler = (name: string, handler: EventHandler) => () => void;

const mocks = vi.hoisted(() => {
  const unsubscribe = vi.fn();
  const on = vi.fn<OnHandler>(() => unsubscribe);
  return { on, unsubscribe };
});

vi.mock("@wailsio/runtime", () => ({
  Events: { On: mocks.on },
}));

import { onAudioState, onBrowseNavigate, onPlayerState, onSkinsChanged } from "./events";

function handlerFor(name: string): EventHandler {
  const call = mocks.on.mock.calls.find(([eventName]) => eventName === name);
  if (!call) throw new Error(`no subscription for ${name}`);
  return call[1];
}

describe("api events", () => {
  beforeEach(() => {
    mocks.on.mockClear();
    mocks.unsubscribe.mockClear();
  });

  it("unwraps player state data and returns the unsubscribe function", () => {
    const listener = vi.fn();
    const unsubscribe = onPlayerState(listener);
    const payload = { provider: "demo", revision: 7 };

    handlerFor("player:state")({ data: payload });

    expect(listener).toHaveBeenCalledWith(payload);
    expect(unsubscribe).toBe(mocks.unsubscribe);
  });

  it("unwraps audio and skin payloads", () => {
    const audio = vi.fn();
    const skins = vi.fn();
    onAudioState(audio);
    onSkinsChanged(skins);

    handlerFor("audio:state")({ data: { available: true } });
    handlerFor("skins:changed")({ data: { activeId: "clarity" } });

    expect(audio).toHaveBeenCalledWith({ available: true });
    expect(skins).toHaveBeenCalledWith({ activeId: "clarity" });
  });

  it("unwraps Browse navigation section names", () => {
    const listener = vi.fn();
    onBrowseNavigate(listener);

    handlerFor("browse:navigate")({ data: "podcasts" });

    expect(listener).toHaveBeenCalledWith("podcasts");
  });

  it("unsubscribes through the returned function", () => {
    const unsubscribe = onPlayerState(vi.fn());
    unsubscribe();
    expect(mocks.unsubscribe).toHaveBeenCalledOnce();
  });
});
