# Pixiu 商业 License 说明

Pixiu 采用 **Ed25519 离线签发的 license 文件** 做商业授权：服务端内嵌验签公钥，客户侧放置 `license.key` 即可解锁对应版本的功能与配额；未配置 license 时自动运行社区版。

## 版本与功能划分

| 能力 | 社区版（默认） | 专业版 | 企业版 |
|---|---|---|---|
| 集群/主机/代理/租户角色等基础能力 | ✅（≤3 集群 / ≤50 主机） | ✅（按 license 配额） | ✅（按 license 配额） |
| 备份恢复（backup） | ❌ | ✅ | ✅ |
| 告警中心（alert） | ❌ | ✅ | ✅ |
| 扩展/定时扩缩容（extension） | ❌ | ✅ | ✅ |
| 多集群分发（distributions） | ❌ | ✅ | ✅ |
| 应用商店高级能力（apps） | ❌ | ✅ | ✅ |
| AI 助手（assistant） | ❌ | ❌ | ✅ |

- 付费功能与 URL 前缀的映射登记在 `api/server/middleware/license.go`，未授权访问返回 403；
- 集群数配额在创建集群时校验（`pkg/controller/cluster`），主机数配额可通过 `license.Get().CheckNodeQuota()` 接入。

## License 文件格式

一行文本：`<base64url(payload JSON)>.<base64url(signature)>`

载荷字段（`pkg/license.Payload`）：

```json
{
  "customer": "示例客户",
  "edition": "professional",
  "features": ["backup", "alert", "extension", "distribution", "apps"],
  "max_clusters": 50,
  "max_nodes": 500,
  "expires_at": 1796083200,
  "fingerprint": "818a26dbed61d71a5bf88b124745fb1a"
}
```

| 字段 | 说明 |
|---|---|
| `edition` | `community` / `professional` / `enterprise`；`enterprise` 默认全量放开付费功能 |
| `features` | 授权的付费功能列表（企业版可省略） |
| `max_clusters` / `max_nodes` | 配额上限，0 或缺省表示不限制 |
| `expires_at` | 过期时间（Unix 秒），0 表示永不过期 |
| `fingerprint` | 机器指纹绑定；空表示不绑定部署环境 |

## 签发方操作（私钥持有者）

> 私钥与 `license-keys/` 目录已在 `.gitignore` 排除，**严禁入库、严禁随产品分发**。

```bash
# 1. 生成密钥对（首次，正式商用前务必重新生成）
go run ./cmd/license-gen keygen --dir ./license-keys
# 将输出的公钥 hex 更新到 pkg/license/publickey.go 后重新编译 pixiu-server

# 2. 获取客户环境的机器指纹（客户在其部署机上执行）
go run ./cmd/license-gen print

# 3. 编写载荷 license.json 后签名
go run ./cmd/license-gen sign --key license-keys/ed25519_private.key \
    --in license.json --out license.key --duration 8760h

# 4. 自检
go run ./cmd/license-gen verify --pub license-keys/ed25519_public.key --in license.key

# 5. 将 license.key 交付客户
```

## 客户侧部署

方式一：将 `license.key` 放到服务端路径（默认 `/etc/pixiu/license.key`，可用 `config.yaml` 的 `license.path` 调整），重启或等待热更新轮询生效。

方式二：管理员登录控制台，调用 `POST /pixiu/license`（body：`{"content": "<license.key 文件内容>"}`）即时生效，验签失败自动回滚旧文件。

查询当前授权状态：`GET /pixiu/license`，返回：

```json
{
  "state": "valid",
  "edition": "professional",
  "customer": "示例客户",
  "features": ["backup", "alert", "extension", "distribution", "apps"],
  "quota": { "max_clusters": 50, "max_nodes": 500 },
  "expires_at": 1796083200,
  "fingerprint": "818a26dbed61d71a5bf88b124745fb1a"
}
```

状态取值：`valid` 有效 / `community` 未配置（社区版）/ `expired` 过期 / `invalid` 验签失败 / `not_effective` 未生效 / `fingerprint_mismatch` 环境不匹配。

## 更换密钥对

1. `go run ./cmd/license-gen keygen` 生成新密钥对；
2. 新公钥 hex 替换 `pkg/license/publickey.go` 中常量并重新编译；
3. 用新私钥重新签发并分发全部客户 license（旧 license 全部失效）。
