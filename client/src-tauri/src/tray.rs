use std::sync::atomic::{AtomicBool, Ordering};
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

pub(crate) fn encode_png(rgba: &[u8], width: u32, height: u32) -> Result<Vec<u8>, String> {
    let img = image::RgbaImage::from_raw(width, height, rgba.to_vec())
        .ok_or_else(|| "invalid image buffer".to_string())?;
    let mut buf = Vec::new();
    image::DynamicImage::ImageRgba8(img)
        .write_to(&mut std::io::Cursor::new(&mut buf), image::ImageFormat::Png)
        .map_err(|e| e.to_string())?;
    Ok(buf)
}

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

static UPLOADING: AtomicBool = AtomicBool::new(false);

fn spawn_upload(app: AppHandle) {
    if UPLOADING.swap(true, Ordering::SeqCst) {
        return;
    }
    std::thread::spawn(move || {
        let _ = upload_clipboard_impl(&app);
        UPLOADING.store(false, Ordering::SeqCst);
    });
}

pub struct HotkeyState(pub Mutex<Option<Shortcut>>);

fn install_shortcut(app: &AppHandle, shortcut: &Shortcut) -> Result<(), String> {
    app.global_shortcut()
        .on_shortcut(shortcut.clone(), |app, _sc, event| {
            if event.state == ShortcutState::Pressed {
                spawn_upload(app.clone());
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
    // Register the new shortcut before dropping the old one, so a failed
    // registration leaves the old hotkey (and its tracked state) intact.
    install_shortcut(app, &shortcut)?;
    if let Some(old) = guard.replace(shortcut) {
        let _ = app.global_shortcut().unregister(old);
    }
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
            "upload" => spawn_upload(app.clone()),
            "quit" => app.exit(0),
            _ => {}
        })
        .build(app)
        .map_err(|e| e.to_string())?;

    let hotkey = match config::load(app) {
        Ok(cfg) => cfg.hotkey,
        Err(e) => {
            eprintln!("config load failed, falling back to default hotkey: {e}");
            "Alt+Shift+V".into()
        }
    };
    update_hotkey(app, &hotkey)?;
    Ok(())
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

    #[test]
    fn parse_valid_hotkey() {
        assert!("Alt+Shift+V".parse::<super::Shortcut>().is_ok());
        assert!("Ctrl+Alt+U".parse::<super::Shortcut>().is_ok());
    }

    #[test]
    fn parse_invalid_hotkey() {
        assert!("NotAKey".parse::<super::Shortcut>().is_err());
    }
}
