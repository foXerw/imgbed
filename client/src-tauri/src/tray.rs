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
