# 客户端托盘 + 全局热键 + 桌面通知 — 设计文档

- 日期：2026-09-04
- 状态：已确认（待实现）
- 关联：`docs/superpowers/specs/2026-09-03-image-hosting-design.md` §5.2.1 / §5.3

## 1. 目标

补齐设计文档 §5.2.1 的头号功能「剪贴板一键上传」：让客户端以**托盘常驻**形态运行，通过**全局热键**在任意应用下截图/复制图片即传，上传后**回填剪贴板 URL 并弹桌面通知**。热键在设置页可自定义。

## 2. 现状与缺口

- 全局热键目前由前端注册（`App.tsx` 的 `register("Ctrl+Shift+U", ...)`），剪贴板读取依赖前端 `readImage()` → canvas。窗口一隐藏，webview 不可靠，热键即失效。
- `global-shortcut` / `clipboard-manager` 插件已安装并注册，但热键从未真正绑定后台能力。
- 托盘、通知、关窗常驻均未实现。

## 3. 已确认的决策

| 决策 | 结论 |
|------|------|
| 关窗行为 | 最小化到托盘常驻，退出仅经托盘菜单 |
| 热键可配置 | 现在即可在设置页配置 |
| 默认热键 | `Alt+Shift+V` |
| 架构 | 热键注册、剪贴板读取、上传、回填、通知全部下沉 Rust 后台 |

## 4. 架构

单一后台命令 `upload_clipboard(app)` 作为唯一入口，被三处复用：全局热键、托盘菜单、前端按钮。这保证窗口隐藏时流程仍完整运行，且逻辑单一来源。

## 5. 模块改动

| 文件 | 改动 |
|------|------|
| `client/src-tauri/src/config.rs` | `ClientConfig` 加 `hotkey: String`（默认 `"Alt+Shift+V"`）；`load` 对旧 JSON 缺字段补默认值 |
| `client/src-tauri/src/tray.rs`（新） | 托盘构建 + 热键注册/重注册 + 后台剪贴板上传 + 关窗拦截 + 通知 |
| `client/src-tauri/src/lib.rs` | 注册 `tauri-plugin-notification`；新增 `upload_clipboard` 命令；`set_config` 扩展 hotkey 并触发重注册；setup 初始化 tray/hotkey/关窗常驻 |
| `client/src/App.tsx` | 设置页加 hotkey 输入框；删前端 `register("Ctrl+Shift+U")`；「上传剪贴板」按钮改调 `invoke("upload_clipboard")` |

## 6. 后台上传数据流

`upload_clipboard(app) -> Result<UploadResult, String>`：

1. `app.clipboard().read_image()` 读剪贴板图片（Rust clipboard-manager API）。
2. RGBA → PNG 字节（优先 tauri `Image::to_png()`；不可用则引入纯 Rust `image` crate，实现时验证）。
3. `upload::upload(&bytes, "clipboard.png", &cfg.server, &cfg.token)`。
4. `app.clipboard().write_text(url)` 回填剪贴板。
5. `history::append(&app, &url)`。
6. 桌面通知：成功 body=url；失败 body=错误信息。

## 7. 热键配置与重注册

- 启动读 config，注册 `hotkey`；当前 `Shortcut` 存 `Mutex<Option<Shortcut>>` 托管状态。
- `set_config(server, token, hotkey)`：存盘；若 hotkey 变更 → unregister 旧 → register 新 → 更新状态。
- 解析失败：报错、不注册、不崩溃。

## 8. 托盘菜单

- 打开主窗口（show + focus）
- 上传剪贴板
- 退出（`app.exit(0)`，绕过关窗拦截）

## 9. 关窗常驻

main 窗口 `on_window_event`：`CloseRequested` → `prevent_close()` + `hide()`。托盘「退出」走 `app.exit(0)` 直接退进程。

## 10. 依赖与权限

- 新增 Cargo 依赖：`tauri-plugin-notification = "2"`。
- capability `default.json` 加 `"notification:default"`。
- 托盘为 Tauri core；剪贴板 / 全局热键已装。

## 11. 测试

- Rust 单测：config 含 hotkey 的 round-trip、默认值、旧 JSON 向后兼容。
- 后台剪贴板/网络流程：编译 + 冒烟（GUI e2e 按需跳过，沿用项目约定）。
- 前端逻辑由后续 Vitest 任务覆盖。

## 12. 范围外（YAGNI）

- 托盘图标自定义/动效。
- 上传进度、多图队列、截图预览。
- 通知点击跳转行为定制。
- 多套热键（仅单热键）。
