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
GET  /pixiu/users/oauth/providers
GET  /pixiu/users/oauth/providers/:provider/config
PUT  /pixiu/users/oauth/providers/:provider/config
GET  /pixiu/users/oauth/providers/:provider/login-url
POST /pixiu/users/oauth/providers/:provider/login
```

飞书兼容接口仍保留：

```text
GET  /pixiu/users/feishu/config
PUT  /pixiu/users/feishu/config
GET  /pixiu/users/feishu/login-url
POST /pixiu/users/feishu/login
```

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
  `config_json` text COMMENT '平台差异化配置，LDAP 等非 OAuth 参数可放这里',
  `auto_create_user` boolean DEFAULT true COMMENT '登录成功且未匹配用户时是否自动创建',
  `default_role` bigint DEFAULT 2 COMMENT '自动创建用户默认角色，1=管理员，2=普通用户',
  `match_email` boolean DEFAULT true COMMENT '是否按邮箱匹配已有 Pixiu 用户',
  `description` text COMMENT '说明',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_oauth_provider` (`provider`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### users 扩展字段

```sql
ALTER TABLE `users`
  ADD COLUMN `feishu_open_id` varchar(128) DEFAULT '' COMMENT '飞书 open_id',
  ADD COLUMN `feishu_union_id` varchar(128) DEFAULT '' COMMENT '飞书 union_id',
  ADD COLUMN `feishu_user_id` varchar(128) DEFAULT '' COMMENT '飞书 user_id',
  ADD COLUMN `avatar_url` varchar(512) DEFAULT '' COMMENT '头像地址',
  ADD COLUMN `source` varchar(32) DEFAULT '' COMMENT '用户来源，如 feishu';

CREATE INDEX `idx_feishu_open_id` ON `users` (`feishu_open_id`);
CREATE INDEX `idx_feishu_union_id` ON `users` (`feishu_union_id`);
CREATE INDEX `idx_feishu_user_id` ON `users` (`feishu_user_id`);
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
默认角色: 建议选择普通用户
邮箱匹配绑定: 建议开启
```

保存后，登录页会自动显示「飞书扫码登录」按钮。

## 权限说明

飞书登录成功后，Pixiu 会按以下顺序查找用户：

1. 通过飞书 `union_id` 匹配已有用户。
2. 通过飞书 `open_id` 匹配已有用户。
3. 如果开启「邮箱匹配绑定」，使用飞书邮箱匹配已有 Pixiu 用户。
4. 如果仍未匹配且开启「自动创建用户」，按配置的「默认角色」创建 Pixiu 用户。

自动创建用户建议使用普通用户角色，不建议默认管理员。

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
