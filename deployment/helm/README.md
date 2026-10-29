# Vibebiz Helm Charts

四个独立 chart 加一个 umbrella chart：

```text
deployment/helm/
  backend/    CP/DP 后端，默认合并部署，也支持 split
  sandbox/    sandbox 与共享工作区
  web/        用户端
  ops-web/    运营后台
  vibebiz/    umbrella，一键部署整套
```

## 前置条件

- Kubernetes 1.27+
- Helm 3
- 可用的 RWX StorageClass（sandbox 工作区）
- 外部 MySQL 8 与 Redis
- 已构建并推送镜像到集群可访问的仓库
- 如启用 Ingress，需要 nginx-ingress 或兼容的 Ingress Controller

## Secret

Chart 不保存任何真实密钥。先创建 Secret：

```bash
kubectl create namespace vibebiz

kubectl -n vibebiz create secret generic vibebiz-backend \
  --from-literal=MYSQL_USERNAME='vibebiz' \
  --from-literal=MYSQL_PASSWORD='change-me' \
  --from-literal=REDIS_PASSWORD='change-me' \
  --from-literal=SANDBOX_REGISTRY_TOKEN='change-me' \
  --from-literal=S3_ACCESS_KEY='change-me' \
  --from-literal=S3_SECRET_KEY='change-me'

kubectl -n vibebiz create secret generic vibebiz-sandbox \
  --from-literal=SANDBOX_REGISTRY_TOKEN='change-me' \
  --from-literal=ANTHROPIC_API_KEY='' \
  --from-literal=ANTHROPIC_AUTH_TOKEN='change-me' \
  --from-literal=LLM_API_KEY=''
```

`sandbox` 与 `backend` 的 `SANDBOX_REGISTRY_TOKEN` 必须一致。

## 安装完整平台

```bash
helm dependency build deployment/helm/vibebiz
helm upgrade --install vibebiz deployment/helm/vibebiz \
  -n vibebiz \
  -f deployment/helm/vibebiz/values.yaml
```

默认：

- backend 使用合并镜像 `vibebiz/backend:local`
- web / ops-web Ingress 关闭
- sandbox 创建 20Gi `ReadWriteMany` PVC
- MySQL / Redis 使用 values 中的地址，不由 chart 安装

## 必改 values

```yaml
backend:
  image:
    repository: registry.example.com/vibebiz/backend
    tag: "0.1.0"
  config:
    mysqlHost: mysql.example.com:3306
    mysqlDatabase: vibebiz
    redisHost: redis.example.com:6379
    corsOrigins: "https://vibebiz.example.com,https://ops.vibebiz.example.com"

sandbox:
  image:
    repository: registry.example.com/vibebiz/sandbox
    tag: "0.1.0"
  config:
    agentMode: claude
  persistence:
    storageClassName: nfs-client

web:
  image:
    repository: registry.example.com/vibebiz/web
    tag: "0.1.0"
  ingress:
    enabled: true
    hosts:
      - host: vibebiz.example.com
        paths:
          - path: /
            pathType: Prefix

ops-web:
  image:
    repository: registry.example.com/vibebiz/ops-web
    tag: "0.1.0"
  ingress:
    enabled: true
    hosts:
      - host: ops.vibebiz.example.com
        paths:
          - path: /
            pathType: Prefix
```

## CP/DP 合并与拆分

默认合并模式：

```yaml
backend:
  mode: combined
```

一个 Deployment 内运行 CP/DP 双端口：

- `backend:8080` → CP
- `backend-dp:8081` → DP

拆分模式：

```yaml
backend:
  mode: split
  splitImages:
    cp:
      repository: registry.example.com/vibebiz/cp
      tag: "0.1.0"
    dp:
      repository: registry.example.com/vibebiz/dp
      tag: "0.1.0"
```

会生成两个 Deployment，Service 名称保持不变，web / sandbox 无需改配置。

对应本地构建：

```bash
make docker-build-split
```

## 单独安装 chart

```bash
helm upgrade --install backend deployment/helm/backend -n vibebiz
helm upgrade --install sandbox deployment/helm/sandbox -n vibebiz
helm upgrade --install web deployment/helm/web -n vibebiz
helm upgrade --install ops-web deployment/helm/ops-web -n vibebiz
```

单独安装时注意：

- backend Service 默认固定为 `backend` / `backend-dp`
- web `upstreams.cp` / `upstreams.dp` 要指向实际 backend Service
- sandbox `config.cpRegistryUrl` 要指向实际 CP Service

## 校验

```bash
helm lint deployment/helm/backend
helm lint deployment/helm/sandbox
helm lint deployment/helm/web
helm lint deployment/helm/ops-web

helm dependency build deployment/helm/vibebiz
helm lint deployment/helm/vibebiz

helm template vibebiz deployment/helm/vibebiz -n vibebiz >/tmp/vibebiz.yaml
helm template vibebiz deployment/helm/vibebiz -n vibebiz --set backend.mode=split >/tmp/vibebiz-split.yaml
```

本仓库不包含 Kubernetes 集群、MySQL/Redis chart、Ingress Controller 或 cert-manager。
