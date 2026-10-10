# 第三方登录接入说明

本文档说明 Pixiu 第三方登录的表结构、接口设计，以及飞书应用创建和后台配置步骤。

## 架构说明

第三方登录统一通过 `oauth_providers` 表管理配置。飞书、企业微信、钉钉、LDAP 等登录源使用相同的配置入口，后端按 `provider` 分发到对应实现。

当前已实现：

- `feishu`：飞书扫码登录

已预留：

- `wechat_work`：企业微信登录
- `dingtalk`：钉钉登录
- `ldap`：LDAP 登录

通用接口：

```text
GET   /pixiu/auth/oauth/providers                     # 管理端，全量（需 root）
GET   /pixiu/auth/oauth/providers/enabled             # 公开，仅启用项（登录页使用）
GET   /pixiu/auth/oauth/providers/:provider/config    # 管理端，查看配置（需 root）
PATCH /pixiu/auth/oauth/providers/:provider/config    # 管理端，部分更新配置（需 root）
POST  /pixiu/auth/oauth/providers/:provider/authorize # 公开，生成第三方授权地址（写入一次性 state 并下发会话 Cookie）
POST  /pixiu/auth/oauth/providers/:provider/exchange  # 公开，凭授权码换取登录态
```

说明：

- `authorize` 由原 `GET .../login-url` 改为 `POST`：该接口会生成一次性 state（5 分钟有效）并下发 OAuth 会话 Cookie，改用 POST 以避免被 GET 缓存/预取等语义误触发；`exchange` 由原 `POST .../login` 更名。
- 管理端接口（`providers`、`/:provider/config`）需 root 权限并持久化；公开接口（`enabled`、`authorize`、`exchange`）不持久化。

## 表结构

项目开启 `default.auto_migrate=true` 时，GORM 会根据 model 自动迁移表结构。手动维护数据库时，可参考下面的 SQL。

### oauth_providers

```sql
CREATE TABLE `oauth_providers` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '主键',
  `gmt_create` datetime DEFAULT NULL COMMENT '创建时间',
  `gmt_modified` datetime DEFAULT NULL COMMENT '修改时间',
  `resource_version` bigint DEFAULT 0 COMMENT '资源版本',
  `provider` varchar(32) NOT NULL COMMENT '登录源标识，如 feishu/wechat_work/dingtalk/ldap',
  `name` varchar(64) DEFAULT '' COMMENT '登录源显示名称',
  `login_type` varchar(32) DEFAULT '' COMMENT '登录类型，如 redirect/password',
  `enabled` boolean DEFAULT false COMMENT '是否启用',
  `app_id` varchar(128) DEFAULT '' COMMENT 'App ID / Client ID',
  `app_secret` varchar(256) DEFAULT '' COMMENT 'App Secret / Client Secret',
  `redirect_uri` varchar(512) DEFAULT '' COMMENT 'OAuth 回调地址',
  `scopes` varchar(512) DEFAULT '' COMMENT 'OAuth 授权范围',
  `config_json` text COMMENT '平台差异化配置，如邮箱域名白名单、LDAP 参数等',
  `auto_create_user` boolean DEFAULT true COMMENT '登录成功且未匹配用户时是否自动创建',
  `match_email` boolean DEFAULT false COMMENT '是否按邮箱匹配已有 Pixiu 用户',
  `description` text COMMENT '说明',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_oauth_provider` (`provider`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### oauth_identities

第三方标识与 Pixiu 用户的绑定关系表。`(provider, subject_type, subject)` 联合唯一索引提供数据库级去重，是跨实例并发的唯一性兜底（内存锁仅在单实例内有效）。`subject_type` 取 `union`（union_id）或 `open`（open_id）。

```sql
CREATE TABLE `oauth_identities` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '主键',
  `gmt_create` datetime DEFAULT NULL COMMENT '创建时间',
  `gmt_modified` datetime DEFAULT NULL COMMENT '修改时间',
  `resource_version` bigint DEFAULT 0 COMMENT '资源版本',
  `user_id` bigint NOT NULL COMMENT '关联的 Pixiu 用户 id',
  `provider` varchar(32) NOT NULL COMMENT '登录源标识，如 feishu',
  `subject_type` varchar(16) NOT NULL COMMENT '第三方标识类型：union|open',
  `subject` varchar(128) NOT NULL COMMENT '第三方标识值（union_id 或 open_id）',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_oauth_subject` (`provider`, `subject_type`, `subject`),
  KEY `idx_oauth_identity_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

说明：自动建号时用户与绑定行在同一事务写入，绑定行唯一冲突会整体回滚，不会产生无绑定行的孤儿用户。`users.oauth_*` 列仍保留作为展示字段，登录查找以 `oauth_identities` 为准。删除用户时，用户行与其 `oauth_identities` 行在同一事务内一并清除，避免残留悬垂绑定行。

升级口径：本功能尚未并入 main，**当前无存量绑定数据**，无需迁移。若未来出现「仅写 `users.oauth_*` 而未写 `oauth_identities`」的历史数据，因登录查找只认 `oauth_identities`，需一次性回填，否则这些第三方身份不会被识别（`subject_type` 取 `union`/`open`）：

```sql
INSERT INTO `oauth_identities` (`user_id`, `provider`, `subject_type`, `subject`, `gmt_create`, `gmt_modified`, `resource_version`)
SELECT `id`, `oauth_provider`, 'union', `oauth_union_id`, NOW(), NOW(), 0
  FROM `users`
 WHERE `oauth_provider` <> '' AND `oauth_union_id` <> '';

INSERT INTO `oauth_identities` (`user_id`, `provider`, `subject_type`, `subject`, `gmt_create`, `gmt_modified`, `resource_version`)
SELECT `id`, `oauth_provider`, 'open', `oauth_open_id`, NOW(), NOW(), 0
  FROM `users`
 WHERE `oauth_provider` <> '' AND `oauth_open_id` <> '';
```

### users 扩展字段

```sql
ALTER TABLE `users`
  ADD COLUMN `oauth_provider` varchar(32) DEFAULT '' COMMENT '第三方登录源标识',
  ADD COLUMN `oauth_open_id` varchar(128) DEFAULT '' COMMENT '第三方 open_id',
  ADD COLUMN `oauth_union_id` varchar(128) DEFAULT '' COMMENT '第三方 union_id',
  ADD COLUMN `oauth_user_id` varchar(128) DEFAULT '' COMMENT '第三方 user_id',
  ADD COLUMN `avatar_url` varchar(512) DEFAULT '' COMMENT '头像地址';

CREATE INDEX `idx_oauth_provider_open_id` ON `users` (`oauth_provider`, `oauth_open_id`);
CREATE INDEX `idx_oauth_provider_union_id` ON `users` (`oauth_provider`, `oauth_union_id`);
```

## 飞书应用创建

1. 进入 [飞书开放平台](https://open.feishu.cn/)，打开「开发者后台」。
2. 创建「企业自建应用」，应用名称可填写 `Pixiu`。
3. 在「凭证与基础信息」中复制：

```text
App ID
App Secret
```

4. 在「安全设置」或「重定向 URL」中添加 Pixiu 回调地址。

本地开发示例：

```text
http://localhost:3006/auth/oauth/feishu/callback
```

局域网访问示例：

```text
http://192.168.30.233:3006/auth/oauth/feishu/callback
```

注意：飞书开放平台、Pixiu 后台配置、浏览器实际访问地址要保持同一套域名/IP 和端口，否则回调校验可能失败。

5. 在「权限管理」中按需申请用户信息权限。建议至少包含：

```text
获取用户基本信息
获取用户邮箱
获取用户手机号
```

如果需要通过邮箱绑定已有 Pixiu 用户，需要申请邮箱相关权限。

6. 发布应用，按企业自建应用流程提交管理员审核或添加测试人员。

## Pixiu 后台配置

进入：

```text
系统管理 -> 第三方登录 -> 飞书
```

填写：

```text
启用登录: 开启
App ID: 飞书应用的 App ID
App Secret: 飞书应用的 App Secret
Redirect URL: http://localhost:3006/auth/oauth/feishu/callback
自动创建用户: 按需开启
邮箱匹配绑定: 默认关闭；如需开启，请在 Config JSON 配置可信邮箱域名白名单
Config JSON: {"email_domains":["example.com"]}
```

保存后，登录页会自动显示「飞书扫码登录」按钮。

## 权限说明

飞书登录成功后，Pixiu 会按以下顺序查找用户：

1. 通过飞书 `union_id` 匹配已有用户。
2. 通过飞书 `open_id` 匹配已有用户。
3. 如果开启「邮箱匹配绑定」且邮箱域名命中 `config_json.email_domains` 白名单，使用飞书邮箱匹配已有 Pixiu 用户（仅允许内置「普通角色」用户，管理员或非普通角色账号不允许通过邮箱自动绑定）。
4. 如果仍未匹配且开启「自动创建用户」，创建 Pixiu 用户。

自动创建的用户一律使用系统内置的「普通角色」及其所属租户，不支持指定管理员等其它角色（已移除 `default_role` 配置项）。

本地开发时如果 `config.yaml` 中 `default.mode=debug`，后端会把请求按 root 用户处理，看到的权限会比真实权限更大。验证真实权限时请使用：

```yaml
default:
  mode: release
```

## 安全注意事项

- 不要提交真实的 `App Secret`、数据库密码、JWT key。
- 本地开发配置建议放在 `config.local.yaml`，该文件已加入 `.gitignore`。
- 生产环境建议使用 HTTPS 回调地址。
- 飞书开放平台中的重定向 URL 必须与 Pixiu 后台配置完全一致。
- 会话 Cookie 的 `Secure` 属性：当请求为 HTTPS（`r.TLS != nil`，或经反向代理时 `X-Forwarded-Proto=https`）时自动置为 `true`，HTTP 下不设置，避免本地开发时 Cookie 被丢弃。
- **进程内单实例限制**：OAuth `state` 会话与建号去重锁均为进程内内存态（`oauthStates` / `oauthIdentityLocks`），仅在单个服务实例内有效。多实例/负载均衡部署时必须使用会话保持（sticky session），或改造为 Redis 等外部共享存储承载 `state`，否则跨实例回跳会因 `state` 校验失败而登录失败。
