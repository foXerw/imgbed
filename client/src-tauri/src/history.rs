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

    #[test]
    fn query_all_empty_on_fresh_db() {
        let conn = rusqlite::Connection::open_in_memory().unwrap();
        init_schema(&conn).unwrap();
        assert!(query_all(&conn).unwrap().is_empty());
    }

    #[test]
    fn insert_then_query_returns_newest_first() {
        let conn = rusqlite::Connection::open_in_memory().unwrap();
        init_schema(&conn).unwrap();
        insert(&conn, "https://x/a.png", "![](https://x/a.png)", "100").unwrap();
        insert(&conn, "https://x/b.png", "![](https://x/b.png)", "200").unwrap();
        let entries = query_all(&conn).unwrap();
        assert_eq!(entries.len(), 2);
        assert_eq!(entries[0].url, "https://x/b.png");
        assert_eq!(entries[0].uploaded_at, "200");
        assert_eq!(entries[1].url, "https://x/a.png");
    }
}

use std::path::PathBuf;
use tauri::{AppHandle, Manager};

fn db_path(app: &AppHandle) -> Result<PathBuf, String> {
    let dir = app.path().app_data_dir().map_err(|e| e.to_string())?;
    Ok(dir.join("history.sqlite"))
}

fn open_db(app: &AppHandle) -> Result<rusqlite::Connection, String> {
    let path = db_path(app)?;
    let conn = rusqlite::Connection::open(&path).map_err(|e| e.to_string())?;
    init_schema(&conn)?;
    Ok(conn)
}

fn init_schema(conn: &rusqlite::Connection) -> Result<(), String> {
    conn.execute(
        "CREATE TABLE IF NOT EXISTS history (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            url TEXT NOT NULL,
            markdown TEXT NOT NULL,
            uploaded_at TEXT NOT NULL
        )",
        [],
    )
    .map_err(|e| e.to_string())?;
    Ok(())
}

fn query_all(conn: &rusqlite::Connection) -> Result<Vec<HistoryEntry>, String> {
    let mut stmt = conn
        .prepare("SELECT url, markdown, uploaded_at FROM history ORDER BY id DESC")
        .map_err(|e| e.to_string())?;
    let rows = stmt
        .query_map([], |row| {
            Ok(HistoryEntry {
                url: row.get(0)?,
                markdown: row.get(1)?,
                uploaded_at: row.get(2)?,
            })
        })
        .map_err(|e| e.to_string())?;
    let mut out = Vec::new();
    for row in rows {
        out.push(row.map_err(|e| e.to_string())?);
    }
    Ok(out)
}

fn insert(
    conn: &rusqlite::Connection,
    url: &str,
    markdown: &str,
    uploaded_at: &str,
) -> Result<(), String> {
    conn.execute(
        "INSERT INTO history (url, markdown, uploaded_at) VALUES (?1, ?2, ?3)",
        rusqlite::params![url, markdown, uploaded_at],
    )
    .map_err(|e| e.to_string())?;
    Ok(())
}

pub fn load(app: &AppHandle) -> Vec<HistoryEntry> {
    match open_db(app) {
        Ok(conn) => query_all(&conn).unwrap_or_default(),
        Err(_) => Vec::new(),
    }
}

pub fn append(app: &AppHandle, url: &str) -> Result<(), String> {
    let conn = open_db(app)?;
    insert(&conn, url, &markdown_for(url), &chrono_now())
}

fn chrono_now() -> String {
    use std::time::{SystemTime, UNIX_EPOCH};
    let secs = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs();
    format!("{}", secs)
}
