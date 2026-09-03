use reqwest::blocking::multipart;
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
        let status = resp.status();
        let body = resp.text().unwrap_or_default();
        return Err(format!("upload failed: {} {}", status, body));
    }
    resp.json::<UploadResult>().map_err(|e| e.to_string())
}
