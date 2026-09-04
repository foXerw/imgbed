# 客户端托盘 + 全局热键 + 桌面通知 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让客户端以托盘常驻形态运行，通过可自定义的全局热键（默认 `Alt+Shift+V`）在任意应用下剪贴板图片即传，上传后回填剪贴板 URL 并弹桌面通知。

**Architecture:** 热键注册、剪贴板读取、上传、回填、通知全部下沉 Rust 后台，收敛为单一命令 `upload_clipboard`，被全局热键、托盘菜单、前端按钮三处复用。关窗时拦截 `CloseRequested` 隐藏到托盘，退出仅经托盘菜单。

**Tech Stack:** Tauri v2（Rust 后端 + React/TS 前端）、`tauri-plugin-notification`、`tauri-plugin-global-shortcut`（已装）、`tauri-plugin-clipboard-manager`（已装）、`image`（RGBA→PNG）。

**Spec:** `docs/superpowers/specs/2026-09-04-client-tray-hotkey-notification-design.md`

## Global Constraints

- 默认全局热键 `Alt+Shift+V`；热键字符串格式 `Modifier+Modifier+Key`（大小写不敏感，`Alt`/`Ctrl`/`Shift` + `V` 等）。
- 配置（服务地址 + token + hotkey）存 `app_data_dir/config.json`；`hotkey` 缺省时反序列化补 `Alt+Shift+V`（旧 config.json 向后兼容）。
- 关窗 = 隐藏到托盘（`prevent_close` + `hide`）；退出仅经托盘菜单 `app.exit(0)`。
- 上传成功/失败均弹桌面通知；成功时 URL 写回剪贴板并追加历史。
- 所有 HTTP 写操作带 `X-Auth-Token` 头；Rust 命令返回 `Result<T, String>`。
- 后台剪贴板/网络流程按项目约定走编译 + 冒烟，不做 GUI e2e 单测。

## 环境备注（执行者必读）

- **Ruling 1**：本机 `api.crates.io` 返回 403，`cargo add`/`tauri add` 会失败。新依赖（`tauri-plugin-notification`、`image`）**直接编辑 `src-tauri/Cargo.toml` 写显式版本**，用 `cargo build` 走 sparse index 拉取。
- **Ruling 4**：插件受 capability 权限门控。`notification` 插件必须在 `capabilities/default.json` 的 `permissions` 加 `"notification:default"`，否则运行时报「not allowed」。
- `tauri` 的 `tray-icon` feature **不在默认集**，用托盘必须显式开启。
- 分支约定：**不在 main 上实现**，从 main 切 feature 分支（或用 worktree）。

---

### Task 1: config 增加 hotkey 字段

**Files:**
- Modify: `client/src-tauri/src/config.rs`

**Interfaces:**
- Consumes: 无
- Produces: `ClientConfig { server: String, token: String, hotkey: String }`，`config::load/save` 签名不变；`Default::hotkey == "Alt+Shift+V"`；旧 JSON 缺 `hotkey` 时补默认值。供 Task 2/3/4 使用。

- [ ] **Step 1: 写失败测试**

在 `config.rs` 的 `tests` 模块替换为：

```rust
#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn default_values() {
        let c = ClientConfig::default();
        assert_eq!(c.server, "http://localhost:8080");
        assert_eq!(c.token, "change-me");
        assert_eq!(c.hotkey, "Alt+Shift+V");
    }

    #[test]
    fn round_trip() {
        let c = ClientConfig {
            server: "https://img.example.com".into(),
            token: "abc".into(),
            hotkey: "Ctrl+Alt+U".into(),
        };
        let s = serde_json::to_string(&c).unwrap();
        let d: ClientConfig = serde_json::from_str(&s).unwrap();
        assert_eq!(c, d);
    }

    #[test]
    fn legacy_config_without_hotkey_defaults() {
        let old = r#"{"server":"https://x.example.com","token":"t"}"#;
        let c: ClientConfig = serde_json::from_str(old).unwrap();
        assert_eq!(c.hotkey, "Alt+Shift+V");
    }
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd client/src-tauri && cargo test config::`
Expected: 编译失败（`ClientConfig` 无 `hotkey` 字段）。

- [ ] **Step 3: 最小实现**

把 `config.rs` 顶部改为：

```rust
use serde::{Deserialize, Serialize};

fn default_hotkey() -> String {
    "Alt+Shift+V".into()
}

#[derive(Serialize, Deserialize, Clone, PartialEq, Debug)]
pub struct ClientConfig {
    pub server: String,
    pub token: String,
    #[serde(default = "default_hotkey")]
    pub hotkey: String,
}

impl Default for ClientConfig {
    fn default() -> Self {
        Self {
            server: "http://localhost:8080".into(),
            token: "change-me".into(),
            hotkey: default_hotkey(),
        }
    }
}
```

（`config_path/load/save` 部分保持不变。）

- [ ] **Step 4: 运行测试确认通过**

Run: `cd client/src-tauri && cargo test config::`
Expected: 3 个测试 PASS。

- [ ] **Step 5: Commit**

```bash
git add client/src-tauri/src/config.rs
git commit -m "feat(client): add configurable hotkey field to config"
```

---

### Task 2: 后台剪贴板上传（含通知）

**Files:**
- Create: `client/src-tauri/src/tray.rs`
- Modify: `client/src-tauri/src/lib.rs`
- Modify: `client/src-tauri/src/Cargo.toml`
- Modify: `client/src-tauri/capabilities/default.json`

**Interfaces:**
- Consumes: `config::load`（Task 1）、`upload::upload`、`history::append`、`UploadResult`。
- Produces: `tray::upload_clipboard_impl(app: &AppHandle) -> Result<UploadResult, String>` 与 `tray::encode_png`（`pub(crate)`）；命令 `upload_clipboard`（供 Task 4 前端按钮调用）。

- [ ] **Step 1: 加依赖与权限**

`Cargo.toml` 的 `[dependencies]` 加：

```toml
tauri-plugin-notification = "2"
image = { version = "0.25", default-features = false, features = ["png"] }
```

`capabilities/default.json` 的 `permissions` 数组加一项：

```json
"notification:default"
```

- [ ] **Step 2: 写失败测试（encode_png）**

创建 `client/src-tauri/src/tray.rs`：

```rust
use tauri::AppHandle;
use tauri_plugin_clipboard_manager::ClipboardExt;
use tauri_plugin_notification::NotificationExt;

use crate::config;
use crate::history;
use crate::upload::{self, UploadResult};

fn encode_png(rgba: &[u8], width: u32, height: u32) -> Result<Vec<u8>, String> {
    let img = image::RgbaImage::from_raw(width, height, rgba.to_vec())
        .ok_or_else(|| "invalid image buffer".to_string())?;
    let mut buf = Vec::new();
    image::DynamicImage::ImageRgba8(img)
        .write_to(&mut std::io::Cursor::new(&mut buf), image::ImageFormat::Png)
        .map_err(|e| e.to_string())?;
    Ok(buf)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn encode_png_roundtrip() {
        // 2x1: 红、蓝两个像素
        let rgba = [255u8, 0, 0, 255, 0, 0, 255, 255];
        let png = encode_png(&rgba, 2, 1).unwrap();
        let decoded = image::load_from_memory(&png).unwrap();
        assert_eq!(decoded.width(), 2);
        assert_eq!(decoded.height(), 1);
    }

    #[test]
    fn encode_png_rejects_bad_buffer() {
        // 3x3 需 36 字节，仅给 4 字节 → from_raw 返回 None
        assert!(encode_png(&[0u8; 4], 3, 3).is_err());
    }
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `cd client/src-tauri && cargo test tray::`
Expected: 编译失败（`image` 依赖尚未加入前，或 `encode_png` 未定义）。若已加依赖，则编译通过、测试失败需先看 Step 1 已就位。

- [ ] **Step 4: 实现后台上传**

在 `tray.rs` 的 `tests` 模块**之前**追加：

```rust
pub fn upload_clipboard_impl(app: &AppHandle) -> Result<UploadResult, String> {
    let result = run_upload(app);
    if let Err(e) = &result {
        let _ = app
            .notification()
            .builder()
            .title("上传失败")
            .body(e.clone())
            .show();
    }
    result
}

fn run_upload(app: &AppHandle) -> Result<UploadResult, String> {
    let img = app.clipboard().read_image().map_err(|e| e.to_string())?;
    let png = encode_png(img.rgba(), img.width(), img.height())?;
    let cfg = config::load(app)?;
    let r = upload::upload(&png, "clipboard.png", &cfg.server, &cfg.token)?;
    let _ = app.clipboard().write_text(r.url.clone());
    let _ = history::append(app, &r.url);
    let _ = app
        .notification()
        .builder()
        .title("上传成功")
        .body(r.url.clone())
        .show();
    Ok(r)
}
```

- [ ] **Step 5: lib.rs 注册命令与插件**

`lib.rs` 顶部 `mod upload;` 之后加 `mod tray;`（无需 `use tray;`，`mod` 声明即可通过 `tray::` 访问）。

在 `get_history` 命令之后加：

```rust
#[tauri::command]
fn upload_clipboard(app: tauri::AppHandle) -> Result<UploadResult, String> {
    tray::upload_clipboard_impl(&app)
}
```

`run()` 中 `global_shortcut` 插件行之后加：

```rust
        .plugin(tauri_plugin_notification::init())
```

`generate_handler![...]` 列表里，`upload_file` 之后加 `upload_clipboard`。

- [ ] **Step 6: 运行测试 + 编译确认**

Run: `cd client/src-tauri && cargo test` 然后 `cargo build`
Expected: 全部测试 PASS；`cargo build` 拉取 `tauri-plugin-notification`、`image` 成功（走 sparse index），编译通过。

- [ ] **Step 7: Commit**

```bash
git add client/src-tauri/src/tray.rs client/src-tauri/src/lib.rs client/src-tauri/Cargo.toml client/src-tauri/capabilities/default.json
git commit -m "feat(client): background clipboard upload with notification"
```

---

### Task 3: 托盘 + 全局热键 + 关窗常驻

**Files:**
- Modify: `client/src-tauri/src/tray.rs`
- Modify: `client/src-tauri/src/lib.rs`
- Modify: `client/src-tauri/src/Cargo.toml`（`tauri` 加 `tray-icon` feature）

**Interfaces:**
- Consumes: `tray::upload_clipboard_impl`（Task 2）、`config::load`（Task 1）。
- Produces: `tray::HotkeyState(pub Mutex<Option<Shortcut>>)`、`tray::init(app) -> Result<(), String>`、`tray::update_hotkey(app, &str) -> Result<(), String>`；`set_config(server, token, hotkey)` 会触发热键重注册。供 Task 4 使用。

- [ ] **Step 1: tauri 开 tray-icon feature**

`Cargo.toml` 的 `tauri` 行改为：

```toml
tauri = { version = "2", features = ["tray-icon"] }
```

- [ ] **Step 2: 写失败测试（热键解析）**

在 `tray.rs` 的 `tests` 模块内追加：

```rust
    #[test]
    fn parse_valid_hotkey() {
        assert!("Alt+Shift+V".parse::<super::Shortcut>().is_ok());
        assert!("Ctrl+Alt+U".parse::<super::Shortcut>().is_ok());
    }

    #[test]
    fn parse_invalid_hotkey() {
        assert!("NotAKey".parse::<super::Shortcut>().is_err());
    }
```

（`super::Shortcut` 指向下方 Step 3 引入的 `use tauri_plugin_global_shortcut::Shortcut;`。）

- [ ] **Step 3: 实现托盘 + 热键 + 关窗常驻**

`tray.rs` 顶部 import 块改为（保留已有 import）：

```rust
use std::sync::Mutex;

use tauri::{
    menu::{Menu, MenuItem},
    tray::TrayIconBuilder,
    AppHandle, Manager, WindowEvent,
};
use tauri_plugin_clipboard_manager::ClipboardExt;
use tauri_plugin_global_shortcut::{GlobalShortcutExt, Shortcut, ShortcutState};
use tauri_plugin_notification::NotificationExt;

use crate::config;
use crate::history;
use crate::upload::{self, UploadResult};
```

`upload_clipboard_impl`/`run_upload` 之后、`tests` 模块之前追加：

```rust
pub struct HotkeyState(pub Mutex<Option<Shortcut>>);

fn install_shortcut(app: &AppHandle, shortcut: &Shortcut) -> Result<(), String> {
    app.global_shortcut()
        .on_shortcut(shortcut.clone(), |app, _sc, event| {
            if event.state == ShortcutState::Pressed {
                let app = app.clone();
                std::thread::spawn(move || {
                    let _ = upload_clipboard_impl(&app);
                });
            }
        })
        .map_err(|e| e.to_string())
}

pub fn update_hotkey(app: &AppHandle, hotkey: &str) -> Result<(), String> {
    let shortcut: Shortcut = hotkey
        .parse()
        .map_err(|e| format!("invalid hotkey {hotkey}: {e}"))?;
    let state = app.state::<HotkeyState>();
    let mut guard = state.0.lock().map_err(|e| e.to_string())?;
    if let Some(old) = guard.take() {
        let _ = app.global_shortcut().unregister(old);
    }
    install_shortcut(app, &shortcut)?;
    *guard = Some(shortcut);
    Ok(())
}

pub fn init(app: &AppHandle) -> Result<(), String> {
    // 关窗常驻：拦截 CloseRequested 隐藏到托盘
    if let Some(window) = app.get_webview_window("main") {
        let w = window.clone();
        window.on_window_event(move |event| {
            if let WindowEvent::CloseRequested { api, .. } = event {
                api.prevent_close();
                let _ = w.hide();
            }
        });
    }

    let show = MenuItem::with_id(app, "show", "打开主窗口", true, None::<&str>)
        .map_err(|e| e.to_string())?;
    let upload = MenuItem::with_id(app, "upload", "上传剪贴板", true, None::<&str>)
        .map_err(|e| e.to_string())?;
    let quit = MenuItem::with_id(app, "quit", "退出", true, None::<&str>)
        .map_err(|e| e.to_string())?;
    let menu = Menu::with_items(app, &[&show, &upload, &quit]).map_err(|e| e.to_string())?;

    TrayIconBuilder::with_id("main-tray")
        .icon(app.default_window_icon().unwrap().clone())
        .menu(&menu)
        .show_menu_on_left_click(true)
        .on_menu_event(|app, event| match event.id().as_ref() {
            "show" => {
                if let Some(w) = app.get_webview_window("main") {
                    let _ = w.show();
                    let _ = w.set_focus();
                }
            }
            "upload" => {
                let app = app.clone();
                std::thread::spawn(move || {
                    let _ = upload_clipboard_impl(&app);
                });
            }
            "quit" => app.exit(0),
            _ => {}
        })
        .build(app)
        .map_err(|e| e.to_string())?;

    let hotkey = config::load(app)
        .map(|c| c.hotkey)
        .unwrap_or_else(|_| "Alt+Shift+V".into());
    update_hotkey(app, &hotkey)?;
    Ok(())
}
```

- [ ] **Step 4: lib.rs 接线 setup + 状态 + set_config 扩展**

`lib.rs` 的 `run()` 改为：

```rust
#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_opener::init())
        .plugin(tauri_plugin_clipboard_manager::init())
        .plugin(tauri_plugin_global_shortcut::Builder::new().build())
        .plugin(tauri_plugin_notification::init())
        .manage(tray::HotkeyState(std::sync::Mutex::new(None)))
        .invoke_handler(tauri::generate_handler![
            get_config,
            set_config,
            upload_bytes,
            upload_file,
            upload_clipboard,
            list_remote,
            delete_remote,
            get_history
        ])
        .setup(|app| {
            let handle = app.handle().clone();
            tray::init(&handle).map_err(std::io::Error::other)?;
            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
```

`set_config` 命令改为：

```rust
#[tauri::command]
fn set_config(
    app: tauri::AppHandle,
    server: String,
    token: String,
    hotkey: String,
) -> Result<(), String> {
    let new_cfg = ClientConfig { server, token, hotkey };
    let old_cfg = config::load(&app)?;
    if new_cfg.hotkey != old_cfg.hotkey {
        tray::update_hotkey(&app, &new_cfg.hotkey)?;
    }
    config::save(&app, &new_cfg)
}
```

- [ ] **Step 5: 运行测试 + 编译确认**

Run: `cd client/src-tauri && cargo test` 然后 `cargo build`
Expected: 全部测试 PASS（含 `parse_valid_hotkey`/`parse_invalid_hotkey`）；`cargo build` 编译通过（`tray-icon` feature 生效）。

- [ ] **Step 6: Commit**

```bash
git add client/src-tauri/src/tray.rs client/src-tauri/src/lib.rs client/src-tauri/Cargo.toml
git commit -m "feat(client): tray icon, global hotkey, close-to-tray"
```

---

### Task 4: 前端设置页（hotkey 输入 + 调后台命令）

**Files:**
- Modify: `client/src/App.tsx`

**Interfaces:**
- Consumes: 命令 `get_config`（返回 `{server,token,hotkey}`）、`set_config`（参数 `{server,token,hotkey}`）、`upload_clipboard`（无参）。
- Produces: 无（终点）。

- [ ] **Step 1: 重写 App.tsx**

替换 `client/src/App.tsx` 全量内容为：

```tsx
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
```

- [ ] **Step 2: 前端构建确认**

Run: `cd client && npm run build`
Expected: 构建成功（无 TS 错误；已移除 `readImage`/`register`/`unregister` 等未用 import）。

- [ ] **Step 3: Commit**

```bash
git add client/src/App.tsx
git commit -m "feat(client): hotkey setting in UI"
```

---

## 收尾验证

- `cd client/src-tauri && cargo build`（完整编译 Tauri 应用，确认 tray-icon + notification + image 全部解析）。
- `cd client && npm run build`（Vite 前端）。
- GUI 冒烟（按项目约定，交互 e2e 可选）：起服务端 → `npm run tauri dev` → 验证托盘图标出现、点 X 隐藏到托盘、按 `Alt+Shift+V` 后剪贴板图片上传并回填 URL + 通知、设置页改热键后新热键生效。
