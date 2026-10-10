# 第三方登录接入说明

本文档说明 Pixiu 第三方登录的表结构、接口设计，以及飞书应用创建和后台配置步骤。

## 架构说明

第三方登录统一通过 `oauth_providers` 表管理配置。飞书、企业微信、钉钉、LDAP 等登录源使用相同的配置入口，后端按 `provider` 分发到对应实现。

当前已实现：

- `feishu`：飞书扫码登录
- `wechat_web`：微信（开放平台「网站应用」）扫码登录
- `wechat_work`：企业微信登录（网站扫码 / 企业微信内网页授权）
- `dingtalk`：钉钉扫码登录

已预留：

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
  `provider` varchar(32) NOT NULL COMMENT '登录源标识，如 feishu/wechat_web/wechat_work/dingtalk/ldap',
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

## 微信（网站应用扫码）

微信登录对接的是**微信开放平台**的「网站应用」（扫码登录），不是「公众平台」的网页授权，也不是企业微信。

### 微信开放平台配置

1. 进入 [微信开放平台](https://open.weixin.qq.com/)，注册开发者账号并创建「网站应用」。
2. 提交应用审核，审核通过后「网站应用」才会下发可用的 `AppID` / `AppSecret`。
3. 在「授权回调域」中填写 Pixiu 的回调域名（**必须为已备案域名**，微信不校验完整路径，只校验域名）：

```text
pixiu.example.com
```

- 授权回调域只填域名，不带 `http(s)://`、端口和路径；Pixiu 后台配置的 `redirect_uri` 其域名必须与之一致。
- 本地 `localhost` 无法用于真机联调（微信要求备案域名），需借助内网穿透或测试域名。

4. 在「授权回调域」对应的网站应用下获取：

```text
AppID
AppSecret
```

5. 本登录使用固定 scope `snsapi_login`（网站应用扫码登录的固定值），无需在开放平台额外勾选。

### Pixiu 后台配置

进入：

```text
系统管理 -> 第三方登录 -> 微信
```

填写：

```text
启用登录: 开启
App ID: 微信开放平台「网站应用」的 AppID
App Secret: 微信开放平台「网站应用」的 AppSecret
Redirect URL: https://pixiu.example.com/auth/oauth/wechat_web/callback
自动创建用户: 按需开启
邮箱匹配绑定: 不可用（见下）
```

保存后，登录页会自动显示「微信登录」按钮。

### 协议流程

1. 前端跳转授权地址：`https://open.weixin.qq.com/connect/qrconnect?appid=...&redirect_uri=...&response_type=code&scope=snsapi_login&state=...#wechat_redirect`（结尾 `#wechat_redirect` 为微信强制要求）。
2. 用户扫码确认后，微信回跳到 `redirect_uri` 并带上 `code`。
3. 后端凭 `code` 调 `sns/oauth2/access_token` 换取 `access_token` 与 `openid`（若开放平台已绑定同主体多应用，还会返回 `unionid`）。
4. 后端再调 `sns/userinfo` 获取昵称（`nickname`）与头像（`headimgurl`）。

### 注意事项

- **不返回邮箱与手机号**：微信网站应用的用户信息接口不返回邮箱和手机号，因此「邮箱匹配绑定（`match_email`）」对本登录源不可用。用户匹配只能依靠 `openid` / `unionid` 绑定，或开启「自动创建用户」。
- **UnionID 需开放平台绑定多应用**：`unionid` 在且仅在开放平台账号下已绑定「网站应用」等应用时返回。未绑定时仅返回 `openid`，此时同一自然人在不同应用下会被视为不同身份。
- **昵称中的 emoji**：微信昵称常含 emoji（4 字节字符），而 Pixiu `users.name` 列为 3 字节 utf8。后端在写入前会丢弃昵称中非 BMP（码点 > 0xFFFF）的字符，避免入库失败；若昵称因此为空，则由建号逻辑回退为 `openid` 派生名。
- **错误以 HTTP 200 返回**：微信接口的业务错误通过响应体里的 `errcode` / `errmsg` 表达（而非 HTTP 状态码），后端已显式判定 `errcode != 0` 并报错。

## 企业微信

企业微信登录对接的是**企业微信管理后台**的自建应用，支持两种登录模式（由 `config_json.login_mode` 选择）：

- `sso`（默认）：**企业微信网站扫码登录**，面向 PC 浏览器（`login.work.weixin.qq.com/wwlogin`）。
- `web`：**企业微信内网页授权**，面向企业微信客户端内置浏览器（`open.weixin.qq.com/connect/oauth2`，scope 固定 `snsapi_base`）。

### 企业微信管理后台配置

1. 进入 [企业微信管理后台](https://work.weixin.qq.com/)，在「我的企业」→「企业信息」中获取：

```text
企业 ID（corpid）
```

2. 在「应用管理」→「自建」中创建应用，进入应用详情获取：

```text
Secret（corpsecret）
AgentId（应用 id）
```

- `sso`（网站扫码登录）模式需要 `AgentId`；`web`（企业微信内网页授权）模式不需要。
- 在应用详情的「企业微信授权登录」→「Web 网页授权及 JS-SDK」或「设置可信域名 / 授权回调域」中，配置 Pixiu 的回调域名与地址。

3. 回调地址示例（与 Pixiu 后台配置的 `redirect_uri` 完全一致）：

```text
https://pixiu.example.com/auth/oauth/wechat_work/callback
```

### Pixiu 后台配置

进入：

```text
系统管理 -> 第三方登录 -> 企业微信
```

填写：

```text
启用登录: 开启
App ID: 企业 ID（corpid）
App Secret: 自建应用的 Secret（corpsecret）
Redirect URL: https://pixiu.example.com/auth/oauth/wechat_work/callback
自动创建用户: 按需开启
邮箱匹配绑定: 需应用具备「读取成员」权限且成员有邮箱（见下）
Config JSON（sso 扫码）: {"agent_id":"1000002","login_mode":"sso"}
Config JSON（网页授权）: {"login_mode":"web"}
```

保存后，登录页会自动显示「企业微信登录」按钮。

### 协议流程

1. 前端跳转授权地址：
   - `sso`：`https://login.work.weixin.qq.com/wwlogin/sso/login?login_type=CorpApp&appid=<corpid>&agentid=<agent_id>&redirect_uri=...&state=...`
   - `web`：`https://open.weixin.qq.com/connect/oauth2/authorize?appid=<corpid>&redirect_uri=...&response_type=code&scope=snsapi_base&state=...#wechat_redirect`（结尾 `#wechat_redirect` 为企业微信强制要求）
2. 用户授权后回跳到 `redirect_uri` 并带上 `code`。
3. 后端用 `corpid`+`corpsecret` 调 `cgi-bin/gettoken` 换取 **corp access_token**（进程内缓存，key=`corpid:corpsecret`，按 `expires_in` 提前 30s 过期）。
4. 后端调 `cgi-bin/auth/getuserinfo` 用 `code` 换取用户身份：**企业成员返回 `userid`**（企业内稳定标识），**非成员/外部联系人返回 `openid`**。
5. （best-effort）若拿到 `userid`，再调 `cgi-bin/user/get` 拉取成员详情（`name` / `mobile` / `email` / `avatar`）用于补全昵称、头像、邮箱、手机号。

### 字段映射

| 企业微信 | Pixiu profile |
|---|---|
| `userid`（企业成员） | `UnionID`（企业内稳定标识，放 union 槽） |
| `openid`（非成员/外部） | `OpenID` |
| 成员详情 `name` | `Name` |
| 成员详情 `avatar` | `AvatarURL` |
| 成员详情 `email` | `Email` |
| 成员详情 `mobile` | `Mobile` |

`userid` 与 `openid` 均为空时报「未返回可绑定的用户标识」。

### 注意事项

- **两种身份来源**：企业成员走 `userid`，非成员/外部联系人只返回 `openid`。两者都写入 `oauth_identities`（`union` / `open`），登录查找按 union → open 顺序匹配。
- **成员详情可能失败**：`cgi-bin/user/get` 对非成员会返回非 0 `errcode`；该步失败会**降级为无详情**（不中断登录），此时 `Name`/`AvatarURL`/`Email`/`Mobile` 为空，由建号逻辑回退为 `userid`/`openid` 派生名。
- **邮箱/手机号需应用权限**：成员详情的 `email`、`mobile` 需应用具备相应读取权限，未授权时为空，「邮箱匹配绑定」将无法命中。
- **错误以 HTTP 200 返回**：企业微信接口的业务错误通过响应体里的 `errcode` / `errmsg` 表达（而非 HTTP 状态码），后端已显式判定 `errcode != 0` 并报错。
- **corp access_token 缓存**：与飞书的 app_token 缓存相互独立（不同 key 空间），按 `corpid:corpsecret` 缓存，避免频繁换取触发企业微信限频。

## 钉钉

钉钉登录对接的是**钉钉开放平台**的「扫码登录」（新版 oauth2 开放能力，v1.0 API），使用「企业内部应用」的 AppKey / AppSecret。

### 钉钉开放平台配置

1. 进入 [钉钉开放平台](https://open-dev.dingtalk.com/)，创建「企业内部应用」。
2. 在「凭证与基础信息」中获取：

```text
Client ID（AppKey）
Client Secret（AppSecret）
```

3. 在应用的「登录与分享」→「回调域名」中配置 Pixiu 的回调地址（须与后台 `redirect_uri` 域名一致）。
4. 回调地址示例：

```text
https://pixiu.example.com/auth/oauth/dingtalk/callback
```

5. 按需在「权限管理」中申请用户信息权限（读取昵称、头像、手机号、邮箱等）。

### Pixiu 后台配置

进入：

```text
系统管理 -> 第三方登录 -> 钉钉
```

填写：

```text
启用登录: 开启
App ID: 企业内部应用的 Client ID（AppKey）
App Secret: 企业内部应用的 Client Secret（AppSecret）
Redirect URL: https://pixiu.example.com/auth/oauth/dingtalk/callback
自动创建用户: 按需开启
邮箱匹配绑定: 需钉钉返回邮箱（见下）
```

保存后，登录页会自动显示「钉钉登录」按钮。

### 协议流程

1. 前端跳转授权地址：`https://login.dingtalk.com/oauth2/auth?redirect_uri=...&response_type=code&client_id=<AppKey>&scope=openid&state=...&prompt=consent`（注意授权地址用的是 `client_id` 参数，而非 `app_id`）。
2. 用户扫码确认后，钉钉回跳到 `redirect_uri` 并带上 `code`。
3. 后端 `POST https://api.dingtalk.com/v1.0/oauth2/userAccessToken`，JSON body `{"clientId","clientSecret","code","grantType":"authorization_code"}`，换取 **userAccessToken**（返回 `accessToken` / `refreshToken` / `expireIn` / `corpId`）。
4. 后端 `GET https://api.dingtalk.com/v1.0/contact/users/me`，请求头 `x-acs-dingtalk-access-token: <accessToken>`，获取用户信息。

### 字段映射

| 钉钉 | Pixiu profile |
|---|---|
| `openId` | `OpenID` |
| `unionId` | `UnionID` |
| `nick` | `Name` |
| `avatarUrl` | `AvatarURL` |
| `email` | `Email` |
| `mobile` | `Mobile` |

`openId` 与 `unionId` 均为空时报「未返回可绑定的用户标识」。

### 注意事项

- **错误以 HTTP 状态码表错**：钉钉 v1.0 API 与微信系（HTTP 200 + errcode）不同，非 2xx 即表示失败，错误体形如 `{"code","message","requestid"}`；后端已按状态码判定并透传 `code`/`message` 摘要。
- **`client_id` 参数名**：授权地址与 token 端点用的是 `client_id`/`clientId`（AppKey），不是 `app_id`；后台配置仍填写在 `App ID` 字段（映射为 AppKey）。
- **邮箱可用性**：`email` 需用户在钉钉侧已绑定邮箱且应用具备相应权限，未返回时「邮箱匹配绑定」无法命中。
- **scope 固定 openid**：扫码登录使用固定 scope `openid`，无需在开放平台额外勾选；`prompt=consent` 强制展示授权确认页。

## 权限说明

所有第三方登录成功后，Pixiu 会按「union_id → open_id → （可选）邮箱 → （可选）自动建号」的顺序查找用户。上述顺序对各登录源通用（飞书、微信、企业微信、钉钉一致）。

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
