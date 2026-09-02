# 图床客户端（Tauri v2）实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现一个 Windows 桌面客户端（Tauri v2），提供剪贴板一键上传、拖拽上传、上传历史与 Markdown 复制、远程图片浏览/删除。

**Architecture:** Tauri v2（Rust 后端 + React/TS 前端）。Rust 侧负责配置读写、HTTP 上传/列表/删除、历史持久化、剪贴板与全局热键；React 侧负责 UI。历史存储用 JSON 文件（`app_data_dir/history.json`，替代 spec 中的 SQLite，见下方说明）。

**Tech Stack:** Tauri v2、React 18、TypeScript、Vite、`reqwest`(blocking)、`tauri-plugin-clipboard-manager`、`tauri-plugin-global-shortcut`、`serde`/`serde_json`。

**Spec:** `docs/superpowers/specs/2026-09-03-image-hosting-design.md`

## 前置依赖

- Node.js ≥ 18、npm。
- Rust 工具链（`rustup`，stable）。
- Windows：Microsoft C++ Build Tools 与 WebView2（Win11 通常已内置）。
- 服务端已可用（按 `docs/superpowers/plans/2026-09-03-image-hosting-server.md` 部署），接口契约：
  - `POST {server}/api/upload`，头 `X-Auth-Token: <token>`，multipart 字段 `file` → `{id,url,filename,size,width,height,ext}`
  - `GET {server}/api/images?page=1&pageSize=50` → `{images:[{id,url,size,uploadedAt}],total,page,pageSize}`
  - `DELETE {server}/api/images/{id}` → 200
  - 错误体 `{"error":"..."}`

## Global Constraints

- 配置（服务地址 + token）存 `app_data_dir/config.json`。
- 历史存 `app_data_dir/history.json`（`Vec<HistoryEntry>`，按上传时间倒序）。
- 复制 Markdown 格式：`![](url)`。
- 所有 HTTP 写操作带 `X-Auth-Token` 头。
- Rust 命令返回 `Result<T, String>`，错误以字符串回传前端。

---

### Task 1: 脚手架

**Files:**
- Create: `client/`（create-tauri-app 生成）
- Modify: `client/package.json`、`client/src-tauri/Cargo.toml`、`client/src-tauri/tauri.conf.json`

**Interfaces:**
- Produces: 可 `npm run tauri dev` 启动的空 Tauri 应用。

- [ ] **Step 1: 生成项目**

```bash
cd /d/code/image
npm create tauri-app@latest client -- --template react-ts --manager npm
```

若出现交互提示：选 `React` + `TypeScript`，包管理器 `npm`。

- [ ] **Step 2: 安装依赖与插件**

```bash
cd client
npm install
npm run tauri add clipboard-manager
npm run tauri add global-shortcut
cargo add reqwest --features blocking,multipart,json -p imgclient
cargo add serde_json -p imgclient
```

> 注：`imgclient` 为 crate 名，以生成的 `Cargo.toml` 实际 `[package] name` 为准；若不同则替换。

- [ ] **Step 3: 验证能启动**

```bash
cd client && npm run tauri dev
```

预期：弹出空窗口。Ctrl+C 退出。

- [ ] **Step 4: 提交**

```bash
cd /d/code/image && git add client && git commit -m "chore(client): scaffold tauri app"
```

---

### Task 2: Rust 配置模块

**Files:**
- Create: `client/src-tauri/src/config.rs`
- Modify: `client/src-tauri/src/lib.rs`（注册命令）
- Test: `client/src-tauri/src/config.rs`（内置单测）

**Interfaces:**
- Produces: `ClientConfig{server: String, token: String}`；`config::load(app) -> Result<ClientConfig, String>`；`config::save(app, &ClientConfig) -> Result<(), String>`。

- [ ] **Step 1: 写失败测试**

创建 `client/src-tauri/src/config.rs`：

```rust
use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize, Clone, PartialEq, Debug)]
pub struct ClientConfig {
    pub server: String,
    pub token: String,
}

impl Default for ClientConfig {
    fn default() -> Self {
        Self {
            server: "http://localhost:8080".into(),
            token: "change-me".into(),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn default_values() {
        let c = ClientConfig::default();
        assert_eq!(c.server, "http://localhost:8080");
        assert_eq!(c.token, "change-me");
    }

    #[test]
    fn round_trip() {
        let c = ClientConfig {
            server: "https://img.example.com".into(),
            token: "abc".into(),
        };
        let s = serde_json::to_string(&c).unwrap();
        let d: ClientConfig = serde_json::from_str(&s).unwrap();
        assert_eq!(c, d);
    }
}
```

- [ ] **Step 2: 运行测试确认通过**

```bash
cd client/src-tauri && cargo test config::tests
```

预期：PASS。

- [ ] **Step 3: 实现读写（依赖 app_data_dir）**

在 `config.rs` 追加：

```rust
use std::fs;
use std::path::PathBuf;
use tauri::{AppHandle, Manager};

fn config_path(app: &AppHandle) -> Result<PathBuf, String> {
    let dir = app
        .path()
        .app_data_dir()
        .map_err(|e| e.to_string())?;
    Ok(dir.join("config.json"))
}

pub fn load(app: &AppHandle) -> Result<ClientConfig, String> {
    let path = config_path(app)?;
    if !path.exists() {
        return Ok(ClientConfig::default());
    }
    let data = fs::read_to_string(&path).map_err(|e| e.to_string())?;
    serde_json::from_str(&data).map_err(|e| e.to_string())
}

pub fn save(app: &AppHandle, cfg: &ClientConfig) -> Result<(), String> {
    let path = config_path(app)?;
    if let Some(dir) = path.parent() {
        fs::create_dir_all(dir).map_err(|e| e.to_string())?;
    }
    let data = serde_json::to_string_pretty(cfg).map_err(|e| e.to_string())?;
    fs::write(&path, data).map_err(|e| e.to_string())
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd client/src-tauri && cargo test
```

预期：PASS。

- [ ] **Step 5: 提交**

```bash
cd /d/code/image && git add client/src-tauri && git commit -m "feat(client): config module"
```

---

### Task 3: Rust 上传模块

**Files:**
- Create: `client/src-tauri/src/upload.rs`
- Modify: `client/src-tauri/src/lib.rs`（注册命令）
- Test: `client/src-tauri/src/upload.rs`（单测：响应解析）

**Interfaces:**
- Consumes: `config::load`。
- Produces: `upload::upload(data: &[u8], filename: &str, server: &str, token: &str) -> Result<UploadResult, String>`；`UploadResult{id,url,filename,size,width,height,ext}`。

- [ ] **Step 1: 写失败测试**

创建 `client/src-tauri/src/upload.rs`：

```rust
use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct UploadResult {
    pub id: String,
    pub url: String,
    pub filename: String,
    pub size: i64,
    pub width: i32,
    pub height: i32,
    pub ext: String,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_upload_response() {
        let body = r#"{"id":"2026/09/03/abc.png","url":"https://img.example.com/2026/09/03/abc.png","filename":"a.png","size":123,"width":100,"height":50,"ext":"png"}"#;
        let r: UploadResult = serde_json::from_str(body).unwrap();
        assert_eq!(r.ext, "png");
        assert_eq!(r.url, "https://img.example.com/2026/09/03/abc.png");
    }
}
```

- [ ] **Step 2: 运行测试确认通过**

```bash
cd client/src-tauri && cargo test upload::tests
```

预期：PASS。

- [ ] **Step 3: 实现上传**

在 `upload.rs` 追加：

```rust
use reqwest::blocking::multipart;

pub fn upload(data: &[u8], filename: &str, server: &str, token: &str) -> Result<UploadResult, String> {
    let url = format!("{}/api/upload", server.trim_end_matches('/'));
    let part = multipart::Part::bytes(data.to_vec())
        .file_name(filename.to_string())
        .mime_str("application/octet-stream")
        .map_err(|e| e.to_string())?;
    let form = multipart::Form::new().part("file", part);

    let client = reqwest::blocking::Client::builder()
        .timeout(std::time::Duration::from_secs(120))
        .build()
        .map_err(|e| e.to_string())?;

    let resp = client
        .post(&url)
        .header("X-Auth-Token", token)
        .multipart(form)
        .send()
        .map_err(|e| e.to_string())?;

    if !resp.status().is_success() {
        let body = resp.text().unwrap_or_default();
        return Err(format!("upload failed: {} {}", resp.status(), body));
    }
    resp.json::<UploadResult>().map_err(|e| e.to_string())
}
```

- [ ] **Step 4: 编译确认**

```bash
cd client/src-tauri && cargo check
```

预期：无错误。

- [ ] **Step 5: 提交**

```bash
cd /d/code/image && git add client/src-tauri && git commit -m "feat(client): upload module"
```

---

### Task 4: Rust 历史模块

**Files:**
- Create: `client/src-tauri/src/history.rs`
- Test: `client/src-tauri/src/history.rs`（内置单测）

**Interfaces:**
- Produces: `HistoryEntry{url: String, markdown: String, uploaded_at: String}`；`history::load(app) -> Vec<HistoryEntry>`；`history::append(app, url: &str) -> Result<(), String>`。

- [ ] **Step 1: 写失败测试**

创建 `client/src-tauri/src/history.rs`：

```rust
use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct HistoryEntry {
    pub url: String,
    pub markdown: String,
    pub uploaded_at: String,
}

pub fn markdown_for(url: &str) -> String {
    format!("![]({})", url)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn markdown_format() {
        assert_eq!(markdown_for("https://x/a.png"), "![](https://x/a.png)");
    }

    #[test]
    fn round_trip() {
        let e = HistoryEntry {
            url: "https://x/a.png".into(),
            markdown: "![](https://x/a.png)".into(),
            uploaded_at: "2026-09-03T00:00:00Z".into(),
        };
        let s = serde_json::to_string(&e).unwrap();
        let d: HistoryEntry = serde_json::from_str(&s).unwrap();
        assert_eq!(e, d);
    }
}
```

- [ ] **Step 2: 运行测试确认通过**

```bash
cd client/src-tauri && cargo test history::tests
```

预期：PASS。

- [ ] **Step 3: 实现持久化**

在 `history.rs` 追加：

```rust
use std::fs;
use std::path::PathBuf;
use tauri::{AppHandle, Manager};

fn history_path(app: &AppHandle) -> Result<PathBuf, String> {
    let dir = app.path().app_data_dir().map_err(|e| e.to_string())?;
    Ok(dir.join("history.json"))
}

pub fn load(app: &AppHandle) -> Vec<HistoryEntry> {
    let Ok(path) = history_path(app) else {
        return Vec::new();
    };
    let Ok(data) = fs::read_to_string(path) else {
        return Vec::new();
    };
    serde_json::from_str(&data).unwrap_or_default()
}

pub fn append(app: &AppHandle, url: &str) -> Result<(), String> {
    let mut entries = load(app);
    entries.insert(
        0,
        HistoryEntry {
            url: url.to_string(),
            markdown: markdown_for(url),
            uploaded_at: chrono_now(),
        },
    );
    let path = history_path(app)?;
    if let Some(dir) = path.parent() {
        fs::create_dir_all(dir).map_err(|e| e.to_string())?;
    }
    let data = serde_json::to_string_pretty(&entries).map_err(|e| e.to_string())?;
    fs::write(path, data).map_err(|e| e.to_string())
}

fn chrono_now() -> String {
    use std::time::{SystemTime, UNIX_EPOCH};
    let secs = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs();
    format!("{}", secs)
}
```

> 注：为避免引入 `chrono` 依赖，用 Unix 秒时间戳字符串；前端展示时再格式化为可读时间。

- [ ] **Step 4: 运行测试确认通过**

```bash
cd client/src-tauri && cargo test
```

预期：PASS。

- [ ] **Step 5: 提交**

```bash
cd /d/code/image && git add client/src-tauri && git commit -m "feat(client): history module"
```

---

### Task 5: Rust 命令层（lib.rs 注册）

**Files:**
- Modify: `client/src-tauri/src/lib.rs`
- Modify: `client/src-tauri/src/main.rs`（若模板不同则保持调用 `.run()`）

**Interfaces:**
- Produces（Tauri 命令）：
  - `get_config(app) -> ClientConfig`
  - `set_config(app, server: String, token: String) -> Result<(), String>`
  - `upload_bytes(app, base64: String, filename: String) -> Result<UploadResult, String>`
  - `upload_file(app, path: String) -> Result<UploadResult, String>`
  - `list_remote(app) -> Result<Vec<RemoteImage>, String>`
  - `delete_remote(app, id: String) -> Result<(), String>`
  - `get_history(app) -> Vec<HistoryEntry>`
  - `RemoteImage{id,url,size,uploadedAt}`

- [ ] **Step 1: 实现命令层**

改写 `client/src-tauri/src/lib.rs`：

```rust
mod config;
mod history;
mod upload;

use serde::{Deserialize, Serialize};
use tauri::Manager;

use config::ClientConfig;
use history::HistoryEntry;
use upload::UploadResult;

#[derive(Serialize, Deserialize, Clone, Debug)]
pub struct RemoteImage {
    pub id: String,
    pub url: String,
    pub size: i64,
    #[serde(rename = "uploadedAt")]
    pub uploaded_at: String,
}

#[tauri::command]
fn get_config(app: tauri::AppHandle) -> ClientConfig {
    config::load(&app).unwrap_or_default()
}

#[tauri::command]
fn set_config(app: tauri::AppHandle, server: String, token: String) -> Result<(), String> {
    config::save(&app, &ClientConfig { server, token })
}

#[tauri::command]
fn upload_bytes(app: tauri::AppHandle, base64: String, filename: String) -> Result<UploadResult, String> {
    use base64::Engine;
    let data = base64::engine::general_purpose::STANDARD
        .decode(base64.trim())
        .map_err(|e| e.to_string())?;
    let cfg = config::load(&app)?;
    let r = upload::upload(&data, &filename, &cfg.server, &cfg.token)?;
    let _ = history::append(&app, &r.url);
    Ok(r)
}

#[tauri::command]
fn upload_file(app: tauri::AppHandle, path: String) -> Result<UploadResult, String> {
    let data = std::fs::read(&path).map_err(|e| e.to_string())?;
    let filename = std::path::Path::new(&path)
        .file_name()
        .and_then(|s| s.to_str())
        .unwrap_or("image.png")
        .to_string();
    let cfg = config::load(&app)?;
    let r = upload::upload(&data, &filename, &cfg.server, &cfg.token)?;
    let _ = history::append(&app, &r.url);
    Ok(r)
}

#[tauri::command]
fn list_remote(app: tauri::AppHandle) -> Result<Vec<RemoteImage>, String> {
    let cfg = config::load(&app)?;
    let url = format!("{}/api/images", cfg.server.trim_end_matches('/'));
    let resp = reqwest::blocking::Client::new()
        .get(&url)
        .header("X-Auth-Token", &cfg.token)
        .send()
        .map_err(|e| e.to_string())?;
    if !resp.status().is_success() {
        return Err(format!("list failed: {}", resp.status()));
    }
    #[derive(Deserialize)]
    struct ListResp {
        images: Vec<RemoteImage>,
    }
    let lr: ListResp = resp.json().map_err(|e| e.to_string())?;
    Ok(lr.images)
}

#[tauri::command]
fn delete_remote(app: tauri::AppHandle, id: String) -> Result<(), String> {
    let cfg = config::load(&app)?;
    let url = format!(
        "{}/api/images/{}",
        cfg.server.trim_end_matches('/'),
        id
    );
    let resp = reqwest::blocking::Client::new()
        .delete(&url)
        .header("X-Auth-Token", &cfg.token)
        .send()
        .map_err(|e| e.to_string())?;
    if !resp.status().is_success() {
        return Err(format!("delete failed: {}", resp.status()));
    }
    Ok(())
}

#[tauri::command]
fn get_history(app: tauri::AppHandle) -> Vec<HistoryEntry> {
    history::load(&app)
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_clipboard_manager::init())
        .plugin(tauri_plugin_global_shortcut::Builder::new().build())
        .invoke_handler(tauri::generate_handler![
            get_config,
            set_config,
            upload_bytes,
            upload_file,
            list_remote,
            delete_remote,
            get_history
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
```

- [ ] **Step 2: 补充依赖并编译**

```bash
cd client/src-tauri
cargo add base64 -p imgclient
cargo add serde --features derive -p imgclient
cargo check
```

预期：无错误（`serde` 若模板已带 `derive` 则跳过）。

- [ ] **Step 3: 提交**

```bash
cd /d/code/image && git add client/src-tauri && git commit -m "feat(client): tauri command layer"
```

---

### Task 6: React 前端 UI

**Files:**
- Modify: `client/src/App.tsx`
- Modify: `client/src/App.css`（可选，简单样式）

**Interfaces:**
- Consumes: Tauri 命令（`invoke`）、`@tauri-apps/plugin-clipboard-manager`、`@tauri-apps/plugin-global-shortcut`。

- [ ] **Step 1: 实现 App.tsx**

改写 `client/src/App.tsx`：

```tsx
import { useEffect, useState, useCallback } from "react";
import { invoke } from "@tauri-apps/api/core";
import { writeText, readImage } from "@tauri-apps/plugin-clipboard-manager";
import { register, unregister } from "@tauri-apps/plugin-global-shortcut";

type UploadResult = {
  id: string; url: string; filename: string; size: number;
  width: number; height: number; ext: string;
};
type RemoteImage = { id: string; url: string; size: number; uploadedAt: string };
type HistoryEntry = { url: string; markdown: string; uploaded_at: string };

type ClipImage = { rgba: Uint8Array | number[]; width: number; height: number };

async function imageToBase64(img: ClipImage): Promise<string> {
  const canvas = document.createElement("canvas");
  canvas.width = img.width;
  canvas.height = img.height;
  const ctx = canvas.getContext("2d")!;
  const data = new Uint8ClampedArray(img.rgba);
  ctx.putImageData(new ImageData(data, img.width, img.height), 0, 0);
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

  const uploadFiles = async (files: FileList) => {
    for (const f of Array.from(files)) {
      try {
        const r = await invoke<UploadResult>("upload_file", { path: (f as any).path });
        setStatus(`已上传: ${r.url}`);
        refreshHistory();
      } catch (e) {
        setStatus(`上传失败: ${String(e)}`);
      }
    }
  };

  const copy = async (text: string) => {
    await writeText(text);
    setStatus("已复制");
  };

  const onDrop = (e: React.DragEvent) => {
    e.preventDefault();
    uploadFiles(e.dataTransfer.files);
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
        onDragOver={(e) => e.preventDefault()}
        onDrop={onDrop}
        style={{ border: "2px dashed #ccc", padding: 24, textAlign: "center", marginBottom: 16 }}
      >
        拖拽图片到这里上传
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
```

- [ ] **Step 2: 编译确认**

```bash
cd client && npm run build
```

预期：构建成功。

- [ ] **Step 3: 提交**

```bash
cd /d/code/image && git add client/src && git commit -m "feat(client): React UI"
```

---

### Task 7: 端到端联调与收尾

**Files:**
- Modify: `client/README.md`（可选）
- Modify: `client/.gitignore`（忽略 node_modules、dist、target）

- [ ] **Step 1: 全量构建**

```bash
cd client && npm run tauri build
```

预期：生成安装包 / 可执行文件，无错误。

- [ ] **Step 2: 联调冒烟**

1. 启动服务端（`server`）。
2. `npm run tauri dev`。
3. 配置服务地址 + token → 保存。
4. 剪贴板截图 → `Ctrl+Shift+U` 上传 → 剪贴板拿到 URL。
5. 拖拽图片 → 上传成功。
6. 历史列表出现记录 → 复制 Markdown 正确为 `![](url)`。
7. 远程图片列表可见 → 删除生效。

- [ ] **Step 3: 提交**

```bash
cd /d/code/image && git add client && git commit -m "chore(client): e2e smoke pass and cleanup"
```

---

## 完成标准

- [ ] `cargo test`（`client/src-tauri`）全部通过。
- [ ] `npm run tauri build` 成功产出 Windows 可执行。
- [ ] 剪贴板 / 拖拽上传、复制 Markdown、历史、远程浏览/删除 全链路联调通过。
- [ ] 服务端地址与 token 可配置并持久化。
