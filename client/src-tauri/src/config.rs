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
