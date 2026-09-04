import { useEffect, useState, useCallback } from "react";
import { invoke } from "@tauri-apps/api/core";
import { getCurrentWebview } from "@tauri-apps/api/webview";
import { writeText } from "@tauri-apps/plugin-clipboard-manager";

type UploadResult = {
  id: string; url: string; filename: string; size: number;
  width: number; height: number; ext: string;
};
type RemoteImage = { id: string; url: string; size: number; uploadedAt: string };
type HistoryEntry = { url: string; markdown: string; uploaded_at: string };
type Config = { server: string; token: string; hotkey: string };

export default function App() {
  const [server, setServer] = useState("");
  const [token, setToken] = useState("");
  const [hotkey, setHotkey] = useState("");
  const [history, setHistory] = useState<HistoryEntry[]>([]);
  const [remote, setRemote] = useState<RemoteImage[]>([]);
  const [status, setStatus] = useState("");

  const refreshHistory = useCallback(async () => {
    try {
      setHistory(await invoke<HistoryEntry[]>("get_history"));
    } catch (e) {
      setStatus(String(e));
    }
  }, []);

  const refreshRemote = useCallback(async () => {
    try {
      setRemote(await invoke<RemoteImage[]>("list_remote"));
    } catch (e) {
      setStatus(String(e));
    }
  }, []);

  useEffect(() => {
    (async () => {
      try {
        const cfg = await invoke<Config>("get_config");
        setServer(cfg.server);
        setToken(cfg.token);
        setHotkey(cfg.hotkey);
        await refreshHistory();
        await refreshRemote();
      } catch (e) {
        setStatus(String(e));
      }
    })();
  }, []);

  useEffect(() => {
    let disposed = false;
    let unlisten: (() => void) | undefined;
    getCurrentWebview().onDragDropEvent(async (event) => {
      if (event.payload.type === "drop") {
        for (const path of event.payload.paths) {
          await uploadPath(path);
        }
      }
    }).then((fn) => {
      if (!disposed) {
        unlisten = fn;
      }
    });
    return () => {
      disposed = true;
      unlisten?.();
    };
  }, []);

  const uploadClipboard = async () => {
    try {
      const r = await invoke<UploadResult>("upload_clipboard");
      setStatus(`已上传并复制: ${r.url}`);
      refreshHistory();
    } catch (e) {
      setStatus(`上传失败: ${String(e)}`);
    }
  };

  const uploadPath = async (path: string) => {
    try {
      const r = await invoke<UploadResult>("upload_file", { path });
      setStatus(`已上传: ${r.url}`);
      refreshHistory();
    } catch (e) {
      setStatus(`上传失败: ${String(e)}`);
    }
  };

  const copy = async (text: string) => {
    try {
      await writeText(text);
      setStatus("已复制");
    } catch (e) {
      setStatus(String(e));
    }
  };

  const saveConfig = async () => {
    try {
      await invoke("set_config", { server, token, hotkey });
      setStatus("配置已保存");
    } catch (e) {
      setStatus(String(e));
    }
  };

  return (
    <main style={{ padding: 16, fontFamily: "system-ui" }}>
      <h1>图床客户端</h1>
      <section style={{ marginBottom: 16 }}>
        <input value={server} onChange={(e) => setServer(e.target.value)} placeholder="服务地址" />
        <input value={token} onChange={(e) => setToken(e.target.value)} placeholder="Token" />
        <input value={hotkey} onChange={(e) => setHotkey(e.target.value)} placeholder="全局热键" />
        <button onClick={saveConfig}>保存配置</button>
        <button onClick={uploadClipboard}>上传剪贴板</button>
      </section>

      <section
        style={{ border: "2px dashed #ccc", padding: 24, textAlign: "center", marginBottom: 16 }}
      >
        拖拽图片到窗口任意位置上传
      </section>

      <div>{status}</div>

      <h2>上传历史</h2>
      <ul>
        {history.map((h, i) => (
          <li key={i}>
            <a href={h.url} target="_blank" rel="noreferrer">{h.url}</a>{" "}
            <button onClick={() => copy(h.url)}>复制 URL</button>{" "}
            <button onClick={() => copy(h.markdown)}>复制 Markdown</button>
          </li>
        ))}
      </ul>

      <h2>远程图片</h2>
      <button onClick={refreshRemote}>刷新</button>
      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill,minmax(140px,1fr))", gap: 8 }}>
        {remote.map((img) => (
          <div key={img.id} style={{ border: "1px solid #ddd", padding: 6 }}>
            <img src={img.url} style={{ width: "100%", height: 100, objectFit: "cover" }} />
            <button onClick={() => copy(img.url)}>URL</button>
            <button
              onClick={async () => {
                if (!confirm(`确认删除 ${img.id}? 此操作不可撤销。`)) return;
                try {
                  await invoke("delete_remote", { id: img.id });
                } catch (e) {
                  setStatus(String(e));
                }
                refreshRemote();
              }}
            >
              删除
            </button>
          </div>
        ))}
      </div>
    </main>
  );
}
