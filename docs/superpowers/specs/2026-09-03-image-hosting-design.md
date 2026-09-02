# 个人图床应用 — 设计文档

- 日期：2026-09-03
- 状态：已确认（待实现）

## 1. 目标

构建一个**个人高效图床**，让博客图片托管在自己的服务器上，并配套一个 **Windows 桌面客户端**用于日常上传、调试图床服务。

两个独立子系统：

1. **图床服务端**（Go）—— 部署在云服务器 / VPS，提供图片上传、静态托管、图片处理、鉴权与 Web 管理界面。
2. **Windows 客户端**（Tauri v2）—— 桌面应用，提供剪贴板一键上传、拖拽上传、历史管理与远程图片浏览/删除，兼具「调试服务」的用途。

## 2. 技术选型

| 组件 | 选型 | 理由 |
|------|------|------|
| 服务端语言 | Go | 单二进制、高并发静态文件服务、VPS 交叉编译部署简单 |
| 服务端框架 | `net/http` + `chi` 路由 | 轻量、中间件友好、贴近标准库 |
| 图片存储 | 本地磁盘 | 最简单、成本最低，按日期分目录 |
| 图片处理 | `disintegration/imaging` + `chai2010/webp` | 纯 Go，无 CGO，便于交叉编译 |
| 客户端框架 | Tauri v2（Rust 后端 + React/TS 前端） | 打包体积小、内存占用低 |
| 客户端前端 | React + TypeScript + Vite | 生态成熟 |
| 客户端本地存储 | SQLite | 持久化上传历史 |
| 部署 | 单二进制 + systemd + Caddy（自动 HTTPS） | 简单可靠 |

## 3. 总体架构

```
┌──────────────────────────┐        HTTPS         ┌──────────────────────────┐
│  Windows 客户端 (Tauri)   │ ──────────────────▶ │   Go 图床服务 (VPS)       │
│  上传 / 浏览 / 复制 URL    │ ◀────────────────── │  上传 / 静态托管 / 管理    │
└──────────────────────────┘                      └───────────┬──────────────┘
                                                              │ 本地磁盘
                                                         ┌────▼────┐
                                                         │ 图片文件 │
                                                         └─────────┘
博客 ──▶ 通过公网 URL 直接引用图片（服务端静态托管，公开可读）
```

- 客户端与服务端通过 HTTP API 交互（JSON + multipart）。
- 博客不直接调用 API，只通过公网 URL 引用静态图片。

## 4. 服务端设计

### 4.1 目录结构

```
server/
├── cmd/imgserver/main.go       # 入口：加载配置、启动 HTTP 服务
├── internal/
│   ├── config/                 # 配置加载与结构体
│   ├── api/                    # HTTP handler 与中间件
│   ├── storage/                # 磁盘存储抽象
│   └── imaging/                # 图片处理（缩放 / 压缩 / WebP）
├── web/                        # 内嵌管理界面（go:embed）
├── config.example.yaml
└── go.mod
```

### 4.2 存储布局

- 图片按上传日期分目录：`storage/YYYY/MM/DD/<8位随机>.<ext>`
- 公网 URL：`{baseURL}/YYYY/MM/DD/<name>`
- 文件名用 8 位随机字符串（字母数字），保留原始扩展名（或转 WebP 后为 `.webp`）。

### 4.3 API 设计

| 方法 | 路径 | 鉴权 | 说明 |
|------|------|------|------|
| POST | `/api/upload` | token | multipart 上传，返回 `{url, filename, size, width, height, ext}` |
| GET | `/api/images` | token | 图片列表，支持分页（`page`、`pageSize`） |
| DELETE | `/api/images/{id}` | token | 按 ID 删除图片文件与记录 |
| GET | `/admin` | token | Web 管理界面（内嵌 HTML） |
| GET | `/{path...}` | 公开 | 静态托管图片 |

**鉴权**：写操作（上传 / 列表 / 删除 / 管理界面）要求 `X-Auth-Token` 头匹配配置中的 token；静态读公开（博客可加载）。

**错误约定**：统一 JSON 错误体 `{ "error": "<message>" }`，HTTP 状态码语义化（400 参数错误、401 未授权、404 不存在、413 超限、500 内部错误）。

**ID 约定**：列表与删除接口中的 `{id}` 即图片文件名（含日期路径，如 `2026/09/03/abc123.png`）。

### 4.4 图片处理

- 上传后：若图片边长超过 `maxDimension`（默认 2560px）或文件大小超过 `maxSize`（默认 5MB），自动缩放压缩。
- `convertWebp`（默认 `false`）：为 `true` 时转 WebP。
- 保留原始格式为默认行为。
- 支持类型白名单：`jpeg`、`png`、`gif`、`webp`、`svg`（可配置）。

### 4.5 配置（config.yaml）

```yaml
listen: ":8080"
storageDir: "./storage"
baseURL: "https://img.example.com"   # 生成公网 URL 用
token: "change-me"                    # 上传/管理鉴权 token
maxDimension: 2560
maxSizeMB: 5
convertWebp: false
allowedTypes: ["jpeg", "png", "gif", "webp", "svg"]
```

## 5. Windows 客户端设计

### 5.1 目录结构

```
client/
├── src/                  # React + TS 前端（UI、上传、历史展示）
├── src-tauri/            # Rust 后端（剪贴板、托盘、全局热键、文件对话框、SQLite）
├── package.json
└── vite.config.ts
```

### 5.2 功能清单

1. **剪贴板一键上传**：托盘图标 + 全局热键；读取剪贴板图片 → 上传 → URL 写回剪贴板 + 通知。
2. **拖拽上传**：拖图片文件到窗口，批量上传，展示结果。
3. **历史 + 复制 Markdown**：本地 SQLite 持久化上传历史；每条记录一键复制 `![](url)` 或纯 URL。
4. **浏览/删除远程图片**：调服务端 `GET /api/images` / `DELETE /api/images/{id}`，在客户端内浏览与删除。

### 5.3 配置

- 服务端地址、token 在客户端设置页配置，持久化到本地。
- 全局热键可自定义。

## 6. 错误处理与测试

- **服务端**：`testing` + `httptest` 覆盖上传、鉴权失败、列表分页、删除、图片处理边界（超限、非法类型、超大文件）。
- **客户端**：前端 Vitest 覆盖上传/复制/历史逻辑；Rust 侧单元测试覆盖剪贴板与存储。

## 7. 部署（VPS）

1. `GOOS=linux GOARCH=amd64 go build` 交叉编译单二进制。
2. 上传至 VPS，systemd 服务管理，数据目录挂载到持久盘。
3. Caddy 反代提供 HTTPS（`img.example.com`），`baseURL` 配置为该域名。

## 8. 范围外（YAGNI）

- 对象存储 / CDN 抽象
- 用户体系 / 多 token
- 图片去重、缩略图多级缓存、CDN
- 客户端账号系统

这些留待后续确有需要时再扩展。
