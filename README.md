# 石头中转站

石头中转站是一个自部署的 AI API 中转站项目，前端使用 Vue/Vite，后端使用 Go。前端构建产物会写入 `backend/internal/web/dist`，后端用 `-tags embed` 编译后会把前端页面嵌入到同一个服务二进制里。

## 目录结构

- `frontend/`：管理后台和用户端前端。
- `backend/`：Go 后端服务，入口是 `backend/cmd/server`。
- `deploy/`：Docker Compose、示例配置和部署脚本。
- `docs/legal/`：前端会读取的合规文档。

## 本地开发

前端开发：

```bash
cd frontend
pnpm install
pnpm run dev
```

前端构建：

```bash
cd frontend
pnpm run build
```

后端本地编译嵌入版二进制：

```bash
cd backend
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags embed -o ../.deploy-build/sub2api-linux-amd64 ./cmd/server
```

## Docker 部署

推荐使用 `deploy/docker-compose.local.yml`，数据会放在 `deploy/data`、`deploy/postgres_data`、`deploy/redis_data`，迁移和备份比较直观。

```bash
cd deploy
cp .env.example .env
mkdir -p data postgres_data redis_data
docker compose -f docker-compose.local.yml up -d
docker compose -f docker-compose.local.yml logs -f sub2api
```

默认访问地址：

```text
http://服务器IP:8080
```

如果要映射到宿主机 `19080`，改 `deploy/.env`：

```env
SERVER_PORT=19080
```

然后重建容器：

```bash
docker compose -f docker-compose.local.yml up -d
```

这里的含义是：容器内部应用仍监听 `8080`，宿主机对外暴露 `19080`。等价于手动运行时的 `-p 19080:8080`。

## 本地构建镜像后上传服务器

适合先在本机构建好镜像，再传到服务器测试或上线。

1. 本地构建 linux/amd64 镜像：

```bash
docker buildx build --platform linux/amd64 -t sub2api-shitou:latest -f Dockerfile --load .
docker save sub2api-shitou:latest | gzip > /tmp/sub2api-shitou-latest.tar.gz
```

2. 上传并导入服务器：

```bash
sshpass -p '<password>' scp -o StrictHostKeyChecking=no /tmp/sub2api-shitou-latest.tar.gz root@<host>:/tmp/
sshpass -p '<password>' ssh -o StrictHostKeyChecking=no root@<host> 'docker load -i /tmp/sub2api-shitou-latest.tar.gz'
```

3. 在服务器用测试端口 `19080` 启动：

```bash
sshpass -p '<password>' ssh root@<host> '
docker stop sub2api-test-app 2>/dev/null || true
docker rm sub2api-test-app 2>/dev/null || true
docker run -d \
  --name sub2api-test-app \
  --restart unless-stopped \
  --network sub2api-test-net \
  -p 19080:8080 \
  -v /root/sub2api-test-data:/app/data \
  sub2api-shitou:latest
docker ps --filter name=sub2api-test-app
'
```

配置文件应放在 `/root/sub2api-test-data/config.yaml`，容器内会通过 `/app/data/config.yaml` 读取。如果服务器没有 `sub2api-test-net`、PostgreSQL、Redis 或这份配置文件，需要先按服务器实际环境创建。测试容器会根据挂载的配置连接对应数据库和 Redis。

## 生产更新流程

上线前先确认当前生产容器、镜像和端口：

```bash
ssh root@<host> '
docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Ports}}\t{{.Status}}"
'
```

本地构建前端和后端：

```bash
cd /path/to/project/frontend
pnpm run build

cd ../backend
mkdir -p ../.deploy-build
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags embed -o ../.deploy-build/sub2api-linux-amd64 ./cmd/server
```

如果服务器是用二进制放进 `/opt/sub2api/build/sub2api` 再构建镜像，可以按这个流程更新：

```bash
sshpass -p '<password>' scp ../.deploy-build/sub2api-linux-amd64 root@<host>:/opt/sub2api/build/sub2api

sshpass -p '<password>' ssh root@<host> '
chmod +x /opt/sub2api/build/sub2api
cd /opt/sub2api/build
podman build -t sub2api-shitou:amd64 .
/opt/sub2api/recreate-app.sh
'
```

`recreate-app.sh` 通常会停止旧应用容器，再用新镜像启动一个新容器。数据库、Redis 和 `/app/data` 不在应用镜像里，只要容器启动参数、网络、挂载和配置文件没有改，新容器会继续连同一套生产数据库。

## 回滚

更新前先记录旧镜像：

```bash
ssh root@<host> '
docker ps --filter name=sub2api --format "{{.Names}} {{.Image}}"
'
```

如果使用 Docker，可以给当前镜像打一个回滚标签：

```bash
ssh root@<host> '
OLD_IMAGE=$(docker ps --filter name=sub2api --format "{{.Image}}" | head -n 1)
ROLLBACK_TAG="sub2api:rollback-$(date +%Y%m%d-%H%M%S)"
docker tag "$OLD_IMAGE" "$ROLLBACK_TAG"
echo "$ROLLBACK_TAG"
'
```

回滚时用旧镜像重新创建应用容器，保持原来的端口、网络、配置和数据卷不变。只要数据库迁移没有做不可逆变更，回滚应用镜像通常是可行的；如果新版本已经改了数据库结构，必须先评估迁移是否兼容，必要时从数据库备份恢复。

## 数据和配置原则

- 应用容器可以随时重建，真正重要的数据在 PostgreSQL、Redis 和 `/app/data` 挂载目录里。
- 生产环境必须固定 `JWT_SECRET` 和 `TOTP_ENCRYPTION_KEY`，否则重启后登录态或 2FA 可能失效。
- 上线前建议备份 PostgreSQL 和 `/app/data`。
- 不要把服务器密码、密钥、支付配置写进仓库；部署文档统一使用 `<password>`、`<host>` 这类占位符。

## 常用检查命令

```bash
# 查看容器
docker ps

# 查看应用日志
docker logs --tail=100 sub2api

# 健康检查
curl -fsS http://127.0.0.1:19080/health

# 前端测试
cd frontend
pnpm test:run src/stores/__tests__/app.spec.ts

# 前端构建
pnpm run build
```
