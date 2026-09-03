mod config;
mod history;
mod upload;

use serde::{Deserialize, Serialize};

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

fn blocking_client() -> Result<reqwest::blocking::Client, String> {
    reqwest::blocking::Client::builder()
        .timeout(std::time::Duration::from_secs(120))
        .build()
        .map_err(|e| e.to_string())
}

#[tauri::command]
fn list_remote(app: tauri::AppHandle) -> Result<Vec<RemoteImage>, String> {
    let cfg = config::load(&app)?;
    let url = format!("{}/api/images", cfg.server.trim_end_matches('/'));
    let resp = blocking_client()?
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
    let resp = blocking_client()?
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
        .plugin(tauri_plugin_opener::init())
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
