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
