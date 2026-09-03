import { useEffect, useState, useCallback } from "react";
import { invoke } from "@tauri-apps/api/core";
import { getCurrentWebview } from "@tauri-apps/api/webview";
import type { Image } from "@tauri-apps/api/image";
import { writeText, readImage } from "@tauri-apps/plugin-clipboard-manager";
import { register, unregister } from "@tauri-apps/plugin-global-shortcut";

type UploadResult = {
  id: string; url: string; filename: string; size: number;
  width: number; height: number; ext: string;
};
type RemoteImage = { id: string; url: string; size: number; uploadedAt: string };
type HistoryEntry = { url: string; markdown: string; uploaded_at: string };

async function imageToBase64(img: Image): Promise<string> {
  const rgba = await img.rgba();
  const size = await img.size();
  const canvas = document.createElement("canvas");
  canvas.width = size.width;
  canvas.height = size.height;
  const ctx = canvas.getContext("2d")!;
  const data = new Uint8ClampedArray(rgba);
  ctx.putImageData(new ImageData(data, size.width, size.height), 0, 0);
  const dataUrl = canvas.toDataURL("image/png");
  return dataUrl.replace(/^data:image\/png;base64,/, "");
}

export default function App() {
  const [server, setServer] = useState("");
  const [token, setToken] = useState("");
  const [history, setHistory] = useState<HistoryEntry[]>([]);
  const [remote, setRemote] = useState<RemoteImage[]>([]);
  const [status, setStatus] = useState("");

  const refreshHistory = useCallback(async () => {
    setHistory(await invoke<HistoryEntry[]>("get_history"));
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
      const cfg = await invoke<{ server: string; token: string }>("get_config");
      setServer(cfg.server);
      setToken(cfg.token);
      refreshHistory();
      refreshRemote();
    })();
    register("Ctrl+Shift+U", uploadClipboard);
    return () => { unregister("Ctrl+Shift+U"); };
  }, []);

  useEffect(() => {
    let unlisten: (() => void) | undefined;
    getCurrentWebview().onDragDropEvent((event) => {
      if (event.payload.type === "drop") {
        for (const path of event.payload.paths) {
          void uploadPath(path);
        }
      }
    }).then((fn) => { unlisten = fn; });
    return () => { unlisten?.(); };
  }, []);

  const uploadClipboard = async () => {
    try {
      const img = await readImage();
      const b64 = await imageToBase64(img);
      const r = await invoke<UploadResult>("upload_bytes", { base64: b64, filename: "clipboard.png" });
      await writeText(r.url);
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
    await writeText(text);
    setStatus("已复制");
  };

  const saveConfig = async () => {
    await invoke("set_config", { server, token });
    setStatus("配置已保存");
  };

  return (
    <main style={{ padding: 16, fontFamily: "system-ui" }}>
      <h1>图床客户端</h1>
      <section style={{ marginBottom: 16 }}>
        <input value={server} onChange={(e) => setServer(e.target.value)} placeholder="服务地址" />
        <input value={token} onChange={(e) => setToken(e.target.value)} placeholder="Token" />
        <button onClick={saveConfig}>保存配置</button>
        <button onClick={uploadClipboard}>上传剪贴板 (Ctrl+Shift+U)</button>
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
                await invoke("delete_remote", { id: img.id });
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
