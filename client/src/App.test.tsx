import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

const { invokeMock, writeTextMock } = vi.hoisted(() => ({
  invokeMock: vi.fn<(cmd: string, args?: unknown) => Promise<unknown>>(),
  writeTextMock: vi.fn<(text: string) => Promise<void>>(),
}));

vi.mock("@tauri-apps/api/core", () => ({ invoke: invokeMock }));
vi.mock("@tauri-apps/plugin-clipboard-manager", () => ({ writeText: writeTextMock }));
vi.mock("@tauri-apps/api/webview", () => ({
  getCurrentWebview: () => ({ onDragDropEvent: () => Promise.resolve(() => {}) }),
}));

import App from "./App";

const CONFIG = { server: "http://img.test", token: "secret", hotkey: "Alt+Shift+V" };

function mockInvoke(overrides: Record<string, unknown> = {}) {
  invokeMock.mockImplementation((cmd) => {
    if (cmd in overrides) return Promise.resolve(overrides[cmd]);
    if (cmd === "get_config") return Promise.resolve(CONFIG);
    if (cmd === "get_history") return Promise.resolve([]);
    if (cmd === "list_remote") return Promise.resolve([]);
    return Promise.resolve(undefined);
  });
}

beforeEach(() => {
  invokeMock.mockReset();
  writeTextMock.mockReset();
  mockInvoke();
});

afterEach(cleanup);

describe("App", () => {
  it("loads config on mount and populates inputs", async () => {
    render(<App />);
    await screen.findByDisplayValue("http://img.test");
    await screen.findByDisplayValue("secret");
    await screen.findByDisplayValue("Alt+Shift+V");
  });

  it("saves config with the hotkey", async () => {
    render(<App />);
    await screen.findByDisplayValue("http://img.test");
    fireEvent.change(screen.getByPlaceholderText("全局热键"), {
      target: { value: "Ctrl+Alt+U" },
    });
    fireEvent.click(screen.getByText("保存配置"));
    await waitFor(() =>
      expect(invokeMock).toHaveBeenCalledWith("set_config", {
        server: "http://img.test",
        token: "secret",
        hotkey: "Ctrl+Alt+U",
      }),
    );
  });

  it("renders upload history from get_history", async () => {
    mockInvoke({
      get_history: [
        { url: "https://x/a.png", markdown: "![](https://x/a.png)", uploaded_at: "100" },
      ],
    });
    render(<App />);
    expect(await screen.findByText("https://x/a.png")).toBeTruthy();
  });

  it("copies URL via writeText", async () => {
    mockInvoke({
      get_history: [
        { url: "https://x/a.png", markdown: "![](https://x/a.png)", uploaded_at: "100" },
      ],
    });
    render(<App />);
    await screen.findByText("https://x/a.png");
    fireEvent.click(screen.getByText("复制 URL"));
    await waitFor(() => expect(writeTextMock).toHaveBeenCalledWith("https://x/a.png"));
  });
});
