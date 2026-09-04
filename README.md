# imgbed

个人图床服务：把博客图片托管在自己的服务器上。Go 服务端 + Windows 桌面客户端。

## 结构

```
server/   Go 服务端：上传、静态托管、图片处理、Token 鉴权、内嵌管理界面
docs/     设计文档与实现计划（specs / plans）
client/   Windows 桌面客户端（Tauri v2）
```

## 服务端快速开始

```bash
cd server
cp config.example.yaml config.yaml   # 修改 token、baseURL
go run ./cmd/imgserver -config config.yaml
```

详细接口与交叉编译见 [`server/README.md`](server/README.md)。

## 客户端

Windows 桌面客户端（Tauri v2）：剪贴板一键上传（托盘 + 全局热键 + 桌面通知）、拖拽上传、上传历史（SQLite）、远程图片浏览/删除。详见 [`client/README.md`](client/README.md)。

## 服务端功能

- 上传（`POST /api/upload`，multipart，需 `X-Auth-Token`）
- 列表 / 删除（`GET /api/images`、`DELETE /api/images/{id}`，需 token）
- 静态托管图片（公开可读，供博客引用）
- 图片处理：超限自动缩放、可选 WebP 转换、格式白名单
- 内嵌 Web 管理界面（`GET /admin`，需 token）

## 服务端安全

- 上传/列表/删除/管理界面均要求 `X-Auth-Token` 头匹配配置 token。
- 启动时校验：`token` 为空会拒绝启动；使用占位 token `change-me` 会告警。
- 上传体积与像素总数均有上限，防解压炸弹与临时磁盘耗尽。
- 静态目录不输出目录列表，避免图片 ID 被枚举。

## 许可证

[MIT](LICENSE)
