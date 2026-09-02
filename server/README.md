# 图床服务端

Go 实现的个人图床服务。上传、静态托管、图片处理、Token 鉴权与内嵌管理界面。

## 运行

    go run ./cmd/imgserver -config config.yaml

## 配置

复制 `config.example.yaml` 为 `config.yaml` 并修改 token、baseURL。

## 接口

- `POST /api/upload`（multipart `file`，需 `X-Auth-Token`）
- `GET /api/images?page=1&pageSize=50`（需 token）
- `DELETE /api/images/{id}`（需 token）
- `GET /admin`（需 token）
- `GET /{path}`（公开静态图片）

## 交叉编译（VPS）

    GOOS=linux GOARCH=amd64 go build -o imgserver ./cmd/imgserver
