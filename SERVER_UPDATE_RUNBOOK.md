# 石头中转站服务器更新流程

本文档记录本次使用的完整更新流程：本地构建镜像、上传服务器、在 `19080` 测试、准备生产上线脚本、上线与回滚。敏感信息不要写进文档，命令中的 `<host>`、`<password>` 按实际环境替换。

## 当前服务器约定

- 服务器：`47.252.50.237`
- 生产容器：`sub2api`
- 生产端口：宿主机 `8090` -> 容器内 `8080`
- 测试容器：`sub2api-test-app`
- 测试端口：宿主机 `19080` -> 容器内 `8080`
- 生产数据卷：`deploy_sub2api_data:/app/data`
- 生产 Redis 网络：`deploy_sub2api-network`
- 生产数据库网络：`lct_default`
- 生产 Redis 主机名：`sub2api-redis`
- 生产 PostgreSQL 主机名：`lct-db-1`

生产 app 容器需要同时连接 `deploy_sub2api-network` 和 `lct_default`。只连接一个网络会导致 Redis 或数据库不可达。

## 1. 本地构建镜像

在项目根目录执行：

```bash
docker buildx build --platform linux/amd64 -t sub2api-shitou:latest -f Dockerfile --load .
docker images sub2api-shitou:latest
```

打包镜像：

```bash
docker save sub2api-shitou:latest | gzip > /tmp/sub2api-shitou-latest.tar.gz
ls -lh /tmp/sub2api-shitou-latest.tar.gz
```

## 2. 上传并导入服务器

```bash
sshpass -p '<password>' scp -o StrictHostKeyChecking=no \
  /tmp/sub2api-shitou-latest.tar.gz \
  root@47.252.50.237:/tmp/sub2api-shitou-latest.tar.gz

sshpass -p '<password>' ssh -o StrictHostKeyChecking=no root@47.252.50.237 '
docker load -i /tmp/sub2api-shitou-latest.tar.gz
docker tag sub2api-shitou:latest sub2api-shitou:test-19080
'
```

## 3. 先上线到 19080 测试

替换测试容器前，给旧测试镜像打回滚标签：

```bash
sshpass -p '<password>' ssh root@47.252.50.237 '
TS=$(date +%Y%m%d-%H%M%S)
docker tag sub2api-shitou:test-19080 sub2api-shitou:test-19080-rollback-$TS 2>/dev/null || true
docker stop sub2api-test-app 2>/dev/null || true
docker rm sub2api-test-app 2>/dev/null || true

docker run -d \
  --name sub2api-test-app \
  --restart unless-stopped \
  --network sub2api-test-net \
  -p 19080:8080 \
  -v /root/sub2api-test-data:/app/data \
  sub2api-shitou:test-19080
'
```

检查测试环境：

```bash
sshpass -p '<password>' ssh root@47.252.50.237 '
docker ps --filter name=sub2api-test-app
curl -fsS http://127.0.0.1:19080/health
docker logs --tail=80 sub2api-test-app
'
```

浏览器访问：

```text
http://47.252.50.237:19080
```

确认页面、登录、核心功能没问题后，再准备生产。

## 4. 生产上线前准备

准备工作只打标签和生成脚本，不重启生产。

```bash
sshpass -p '<password>' ssh root@47.252.50.237 '
TS=$(date +%Y%m%d-%H%M%S)
PREP_DIR=/root/sub2api-prod-prep-$TS
mkdir -p "$PREP_DIR"

docker inspect sub2api > "$PREP_DIR/prod-container.inspect.json"
docker inspect sub2api:v0136-prod > "$PREP_DIR/prod-image.inspect.json" 2>/dev/null || true
docker inspect sub2api-shitou:latest > "$PREP_DIR/candidate-image.inspect.json"
docker ps --filter name="^/sub2api$" \
  --format "table {{.Names}}\t{{.Image}}\t{{.Ports}}\t{{.Status}}" \
  > "$PREP_DIR/prod-container.ps.txt"

docker tag sub2api:v0136-prod sub2api:prod-rollback-$TS
docker tag sub2api-shitou:latest sub2api-shitou:prod-candidate-$TS
docker tag sub2api-shitou:latest sub2api-shitou:prod-candidate

cat > "$PREP_DIR/prod-env.list" <<EOF
PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
EOF

cat > "$PREP_DIR/SUMMARY.txt" <<EOF
prepared_at=$TS
prod_container=sub2api
rollback_tag=sub2api:prod-rollback-$TS
candidate_tag=sub2api-shitou:prod-candidate-$TS
candidate_alias=sub2api-shitou:prod-candidate
primary_network=deploy_sub2api-network
extra_networks=lct_default
port_args=-p 8090:8080
mount_args=-v deploy_sub2api_data:/app/data
start_strategy=docker_create_connect_networks_then_start
prod_was_not_restarted=true
EOF

echo "$PREP_DIR"
'
```

实际这次准备目录是：

```text
/root/sub2api-prod-prep-20260620-224450
```

## 5. 生成生产上线脚本

注意：生产容器需要先 `docker create`，再连接 `lct_default`，最后 `docker start`。这样应用启动时已经能同时访问 Redis 和数据库。

```bash
cat > /root/sub2api-prod-prep-20260620-224450/deploy-prod.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
PROD_CONTAINER="sub2api"
PRIMARY_NETWORK="deploy_sub2api-network"
EXTRA_NETWORKS=("lct_default")
ENV_FILE="/root/sub2api-prod-prep-20260620-224450/prod-env.list"
CANDIDATE_TAG="sub2api-shitou:prod-candidate-20260620-224450"
PORT_ARGS=("-p" "8090:8080")
MOUNT_ARGS=("-v" "deploy_sub2api_data:/app/data")

docker stop "$PROD_CONTAINER"
docker rm "$PROD_CONTAINER"

docker create \
  --name "$PROD_CONTAINER" \
  --restart unless-stopped \
  --network "$PRIMARY_NETWORK" \
  --env-file "$ENV_FILE" \
  "${PORT_ARGS[@]}" \
  "${MOUNT_ARGS[@]}" \
  "$CANDIDATE_TAG" >/dev/null

for NET in "${EXTRA_NETWORKS[@]}"; do
  docker network connect "$NET" "$PROD_CONTAINER"
done

docker start "$PROD_CONTAINER" >/dev/null

for i in $(seq 1 60); do
  HEALTH=$(docker inspect -f "{{.State.Health.Status}}" "$PROD_CONTAINER" 2>/dev/null || echo missing)
  echo "health=$HEALTH"
  if [ "$HEALTH" = healthy ]; then break; fi
  sleep 2
done

curl -fsS --max-time 10 http://127.0.0.1:8090/health
EOF

chmod +x /root/sub2api-prod-prep-20260620-224450/deploy-prod.sh
```

## 6. 生成回滚脚本

```bash
cat > /root/sub2api-prod-prep-20260620-224450/rollback-prod.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
PROD_CONTAINER="sub2api"
PRIMARY_NETWORK="deploy_sub2api-network"
EXTRA_NETWORKS=("lct_default")
ENV_FILE="/root/sub2api-prod-prep-20260620-224450/prod-env.list"
ROLLBACK_TAG="sub2api:prod-rollback-20260620-224450"
PORT_ARGS=("-p" "8090:8080")
MOUNT_ARGS=("-v" "deploy_sub2api_data:/app/data")

docker stop "$PROD_CONTAINER" || true
docker rm "$PROD_CONTAINER" || true

docker create \
  --name "$PROD_CONTAINER" \
  --restart unless-stopped \
  --network "$PRIMARY_NETWORK" \
  --env-file "$ENV_FILE" \
  "${PORT_ARGS[@]}" \
  "${MOUNT_ARGS[@]}" \
  "$ROLLBACK_TAG" >/dev/null

for NET in "${EXTRA_NETWORKS[@]}"; do
  docker network connect "$NET" "$PROD_CONTAINER"
done

docker start "$PROD_CONTAINER" >/dev/null

for i in $(seq 1 60); do
  HEALTH=$(docker inspect -f "{{.State.Health.Status}}" "$PROD_CONTAINER" 2>/dev/null || echo missing)
  echo "health=$HEALTH"
  if [ "$HEALTH" = healthy ]; then break; fi
  sleep 2
done

curl -fsS --max-time 10 http://127.0.0.1:8090/health
EOF

chmod +x /root/sub2api-prod-prep-20260620-224450/rollback-prod.sh
```

## 7. 上线前复核

```bash
sshpass -p '<password>' ssh root@47.252.50.237 '
bash -n /root/sub2api-prod-prep-20260620-224450/deploy-prod.sh
bash -n /root/sub2api-prod-prep-20260620-224450/rollback-prod.sh

docker ps --filter name="^/sub2api$" \
  --format "table {{.Names}}\t{{.Image}}\t{{.Networks}}\t{{.Status}}"

curl -fsS http://127.0.0.1:8090/health
curl -fsS http://127.0.0.1:19080/health

docker run --rm \
  --network deploy_sub2api-network \
  -v deploy_sub2api_data:/app/data \
  --entrypoint sh \
  sub2api-shitou:prod-candidate-20260620-224450 \
  -c "test -f /app/data/config.yaml && echo config_ok"
'
```

额外确认候选容器能连两个依赖网络：

```bash
sshpass -p '<password>' ssh root@47.252.50.237 '
CID=$(docker create \
  --network deploy_sub2api-network \
  -v deploy_sub2api_data:/app/data \
  --entrypoint sh \
  sub2api-shitou:prod-candidate-20260620-224450 \
  -c "test -f /app/data/config.yaml && echo config_ok; getent hosts sub2api-redis; getent hosts lct-db-1; nc -z -w 5 sub2api-redis 6379 && echo redis_tcp_ok; nc -z -w 5 lct-db-1 5432 && echo db_tcp_ok")
docker network connect lct_default "$CID"
docker start -a "$CID"
docker rm "$CID"
'
```

预期输出包含：

```text
config_ok
redis_tcp_ok
db_tcp_ok
```

## 8. 正式上线

会中断生产 app 容器，一般约 `30-60` 秒：

```bash
sshpass -p '<password>' ssh root@47.252.50.237 '
bash /root/sub2api-prod-prep-20260620-224450/deploy-prod.sh
'
```

成功标志：

```text
health=healthy
{"status":"ok"}
```

上线后检查：

```bash
sshpass -p '<password>' ssh root@47.252.50.237 '
docker ps --filter name="^/sub2api$"
curl -fsS http://127.0.0.1:8090/health
docker logs --tail=100 sub2api
'
```

## 9. 回滚

如果上线后发现异常，立即执行：

```bash
sshpass -p '<password>' ssh root@47.252.50.237 '
bash /root/sub2api-prod-prep-20260620-224450/rollback-prod.sh
'
```

回滚后检查：

```bash
sshpass -p '<password>' ssh root@47.252.50.237 '
docker ps --filter name="^/sub2api$"
curl -fsS http://127.0.0.1:8090/health
docker logs --tail=100 sub2api
'
```

## 10. 注意事项

- 不要停止或删除 PostgreSQL、Redis 和 Docker volume。
- 生产数据在 `deploy_sub2api_data:/app/data`，不是 app 镜像里。
- 生产配置的数据库主机是 `lct-db-1`，所以 app 必须连接 `lct_default`。
- 生产 Redis 主机是 `sub2api-redis`，所以 app 必须连接 `deploy_sub2api-network`。
- 上线脚本不要改成直接 `docker run --network deploy_sub2api-network`，否则会漏掉数据库网络。
- 服务器密码不要写入仓库或文档。
