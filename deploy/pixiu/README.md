# Pixiu Kubernetes 安装（对应仓库根目录 install.md 手动步骤）

## 前置

- 可用的 Kubernetes 集群与 `kubectl`
- 能拉取镜像（或已提前导入）：
  - `crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu:v2.0.1-beta.4`
  - `ccr.ccs.tencentyun.com/pixiucloud/mysql:5.7`（使用内置库时）

可选（自建 K8s / runner，对应 install.md）：

```bash
docker pull ccr.ccs.tencentyun.com/pixiucloud/kubez-ansible:v2.0.2
docker pull ccr.ccs.tencentyun.com/pixiucloud/kubez-ansible:v3.0.3
```

## 安装

### 方式 A：一键（含内置 MySQL）

```bash
kubectl apply -f deploy/pixiu/
```

### 方式 B：使用已有数据库

1. 在库中执行 `CREATE DATABASE pixiu;`
2. 编辑 `02-configmap.yaml`，将 `mysql.host/user/password/port/name` 改为实际库信息
3. 跳过 MySQL 清单：

```bash
kubectl apply -f deploy/pixiu/00-namespace.yaml
kubectl apply -f deploy/pixiu/02-configmap.yaml
kubectl apply -f deploy/pixiu/03-pixiu.yaml
```

## 访问

```bash
kubectl -n pixiu-system get svc pixiu
```

浏览器打开：`http://<节点IP>:<NodePort>`

默认账号（与 install.md 一致，可在 ConfigMap 修改）：

- 用户名：`admin`
- 密码：`Pixiu123456!`

## 文件说明

| 文件 | 说明 |
|------|------|
| `00-namespace.yaml` | 命名空间 `pixiu-system` |
| `01-mysql.yaml` | 可选 MySQL（Secret / PVC / Deployment / Service） |
| `02-configmap.yaml` | `/etc/pixiu/config.yaml` |
| `03-pixiu.yaml` | Pixiu Deployment + NodePort Service |

## 卸载

```bash
kubectl delete -f deploy/pixiu/
```

注意：删除 PVC 会清除 MySQL 数据。
