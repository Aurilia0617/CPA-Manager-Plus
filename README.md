# CPA Manager Plus 镜像打包与上传记录

这份文件只记录本项目当前实际使用的镜像构建、推送和 Hugging Face Space 更新方式，方便下次直接执行。

## 目标镜像

```bash
ghcr.io/aurilia0617/cpa-manager-plus:latest
```

当前 Hugging Face Space 使用该镜像运行 CPA Manager Plus。

## 前置 CPA 部署

当前面板连接的 CPA 上游 Space 是：

```text
https://aurilia-oma-basic.hf.space
```

本地部署仓库路径：

```bash
/Users/aurilia/project/oma-basic
```

该 Space 运行 CLIProxyAPI，当前按 `linux/amd64` 镜像摘要固定到 `v7.1.32`。升级 CPA 时先更新这个仓库的 `Dockerfile`，推送后等待 Hugging Face Space 重建，再回到管理面板确认“服务端版本”。

## 架构要求

Hugging Face Space 运行环境是 `linux/amd64`，在 Apple Silicon Mac 上构建时必须指定平台：

```bash
--platform linux/amd64
```

否则容易出现：

```text
exec /usr/local/bin/cpa-manager-plus: exec format error
```

## 代理要求

构建和推送时使用本机 `7897` 端口代理：

```bash
HTTP_PROXY=http://127.0.0.1:7897 HTTPS_PROXY=http://127.0.0.1:7897
```

## 构建并推送镜像

在项目根目录执行：

```bash
cd /Users/aurilia/project/CPA-Manager-Plus

HTTP_PROXY=http://127.0.0.1:7897 HTTPS_PROXY=http://127.0.0.1:7897 \
docker build --platform linux/amd64 \
  -f Dockerfile.manager-server \
  -t ghcr.io/aurilia0617/cpa-manager-plus:latest \
  --build-arg VERSION=$(git rev-parse --short HEAD) \
  .

HTTP_PROXY=http://127.0.0.1:7897 HTTPS_PROXY=http://127.0.0.1:7897 \
docker push ghcr.io/aurilia0617/cpa-manager-plus:latest
```

## 检查镜像架构

```bash
docker image inspect ghcr.io/aurilia0617/cpa-manager-plus:latest \
  --format '{{.Os}}/{{.Architecture}} {{.Id}}'
```

期望结果包含：

```text
linux/amd64
```

## 查看远端摘要

```bash
HTTP_PROXY=http://127.0.0.1:7897 HTTPS_PROXY=http://127.0.0.1:7897 \
docker manifest inspect ghcr.io/aurilia0617/cpa-manager-plus:latest
```

推送成功后记录输出里的 digest，例如：

```text
sha256:xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

## 更新 Hugging Face Space 使用的新镜像

Space 仓库路径：

```bash
/Users/aurilia/project/oma
```

更新 `Dockerfile` 第一行，把镜像固定到最新摘要，避免 Hugging Face 继续使用旧的 `latest` 缓存：

```Dockerfile
FROM ghcr.io/aurilia0617/cpa-manager-plus@sha256:xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

然后提交并通过 `7897` 代理推送：

```bash
cd /Users/aurilia/project/oma

git add Dockerfile
git commit -m "pin cpa manager plus image"
git -c http.proxy=http://127.0.0.1:7897 \
    -c https.proxy=http://127.0.0.1:7897 \
    push origin main
```

推送后去 Hugging Face Space 点：

```text
Factory rebuild
```

## Hugging Face Secrets 建议

如果使用远程 PostgreSQL，例如 Aiven，需要在 Hugging Face Space 的 Secrets 中配置：

```text
DATABASE_URL=postgres://user:password@host:port/dbname?sslmode=require
CPA_MANAGER_DATA_KEY=固定的长随机字符串
CPA_MANAGER_ADMIN_KEY=固定管理员登录密码
```

说明：

- `DATABASE_URL` 存放远程 PostgreSQL 连接串。
- `CPA_MANAGER_DATA_KEY` 必须固定，否则重建后可能无法解密已保存的密钥数据。
- `CPA_MANAGER_ADMIN_KEY` 用来固定登录密码；当前版本支持用它覆盖远程数据库中已有的管理员凭据。
- 管理员密钥不会再打印到日志里，避免泄露。

## 常用完整流程

```bash
cd /Users/aurilia/project/CPA-Manager-Plus

git status --short

go test ./apps/manager-server/...

HTTP_PROXY=http://127.0.0.1:7897 HTTPS_PROXY=http://127.0.0.1:7897 \
docker build --platform linux/amd64 \
  -f Dockerfile.manager-server \
  -t ghcr.io/aurilia0617/cpa-manager-plus:latest \
  --build-arg VERSION=$(git rev-parse --short HEAD) \
  .

HTTP_PROXY=http://127.0.0.1:7897 HTTPS_PROXY=http://127.0.0.1:7897 \
docker push ghcr.io/aurilia0617/cpa-manager-plus:latest

HTTP_PROXY=http://127.0.0.1:7897 HTTPS_PROXY=http://127.0.0.1:7897 \
docker manifest inspect ghcr.io/aurilia0617/cpa-manager-plus:latest
```

拿到新的 digest 后，更新 `/Users/aurilia/project/oma/Dockerfile` 的 `FROM` 行并推送。