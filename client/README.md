# Image Hosting Client

Tauri v2 desktop client (Rust backend + React/TypeScript frontend) for the image hosting service.

## Features

- Configure server address, token, and global hotkey (persisted locally).
- Clipboard one-click upload via a global hotkey (default `Alt+Shift+V`) or the tray menu — URL copied back to clipboard and a desktop notification shown.
- System tray with open / upload / quit; closing the window minimizes to tray.
- Native drag-and-drop image upload.
- Upload history (SQLite-backed) with copy URL / Markdown.
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
cd src-tauri && cargo test   # Rust unit tests
npm test                     # Vitest frontend tests
```
