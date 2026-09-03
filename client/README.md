# Image Hosting Client

Tauri v2 desktop client (Rust backend + React/TypeScript frontend) for the image hosting service.

## Features

- Configure server address and token (persisted locally).
- Clipboard screenshot upload via `Ctrl+Shift+U` (URL copied back to clipboard).
- Native drag-and-drop image upload.
- Upload history with copy URL / Markdown.
- Remote image browse and delete.

## Development

```bash
npm install
npm run tauri dev
```

## Build

```bash
npm run tauri build
```

## Test

```bash
cd src-tauri && cargo test
```
