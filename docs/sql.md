# 创建 `pixiu` 数据库
```sql
CREATE DATABASE pixiu;
```

## 创建 `clusters` 表
```sql
CREATE TABLE `clusters` (
    id int primary key NOT NULL AUTO_INCREMENT COMMENT '主键' ,
    gmt_create datetime COMMENT '创建时间',
    gmt_modified datetime COMMENT '修改时间',
    resource_version int COMMENT '版本号',
    plan_id int COMMENT 'plan表的id号',
    name varchar(128) COMMENT 'k8s 集群名称',
    alias_name varchar(128) COMMENT 'k8s 集群中文名称',
    cluster_type int COMMENT 'Kubernetes 集群的类型',
    status tinyint(4) COMMENT '集群状态',
    kubernetes_version varchar(64) COMMENT 'k8s 集群版本',
    nodes text COMMENT '集群节点详情',
    protected bool COMMENT '集群删除保护',
    kube_config text COMMENT 'kubeConfig 文件内容',
    description text COMMENT 'k8s 集群描述信息',
    extension text COMMENT '扩展预留',
    KEY `idx_name` (`name`),
    UNIQUE KEY `name` (`name`)
) ENGINE=InnoDB CHARSET=utf8 AUTO_INCREMENT=20220801;
```

## 创建 `users` 表
```sql
CREATE TABLE `users` (
    id int primary key NOT NULL AUTO_INCREMENT COMMENT '主键' ,
    gmt_create datetime COMMENT '创建时间',
    gmt_modified datetime COMMENT '修改时间',
    resource_version int COMMENT '版本号',
    name varchar(128) COMMENT '用户名',
    password varchar(256) COMMENT '用户密码',
    email varchar(128) COMMENT '邮箱',
    status tinyint COMMENT '状态: 1启用,2未启用',
    role varchar(128) COMMENT '角色',
    description text COMMENT '描述',
    extension text COMMENT '扩展字段',
    KEY `idx_name` (`name`),
    UNIQUE KEY `name` (`name`)
) ENGINE=InnoDB CHARSET=utf8 AUTO_INCREMENT=21220801;
```

### 创建 `pixiu` 用户
```sql
# 用户 pixiu 的初始密码为 Pixiu123456!
insert into users(name, password) values ('pixiu', '$2a$10$SamcBWw.aPMDv5QadDr7f.2rDBWiwfTwnbh5sEEhaTkWfVwO96PfW');
```

### 第三方登录用户字段
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

## 创建 `oauth_providers` 表
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

## 创建 `tenants` 表
```sql
CREATE TABLE `tenants` (
    id int primary key NOT NULL AUTO_INCREMENT COMMENT '主键' ,
    gmt_create datetime COMMENT '创建时间',
    gmt_modified datetime COMMENT '修改时间',
    resource_version int COMMENT '版本号',
    name varchar(128) COMMENT '租户名',
    description text COMMENT '描述',
    extension text COMMENT '扩展字段',
    KEY `idx_name` (`name`),
    UNIQUE KEY `name` (`name`)
) ENGINE=InnoDB CHARSET=utf8 AUTO_INCREMENT=22220801;
```

## 创建 `roles` 表
```sql
CREATE TABLE `roles` (
    id int primary key NOT NULL AUTO_INCREMENT COMMENT '主键' ,
    gmt_create datetime COMMENT '创建时间',
    gmt_modified datetime COMMENT '修改时间',
    resource_version int COMMENT '版本号',
    name varchar(128) COMMENT '用户名',
    status tinyint(4) COMMENT '状态',
    sequence bigint(20) NOT NULL,
    parent_id bigint(20) NOT NULL,
    memo varchar(128) DEFAULT NULL,
    KEY `idx_name` (`name`),
    UNIQUE KEY `name` (`name`)
) ENGINE=InnoDB CHARSET=utf8 AUTO_INCREMENT=23220801;
```

## 创建 `clouds` 表
```sql
CREATE TABLE `clouds` (
    id int primary key NOT NULL AUTO_INCREMENT COMMENT '主键' ,
    gmt_create datetime COMMENT '创建时间',
    gmt_modified datetime COMMENT '修改时间',
    resource_version int COMMENT '版本号',
    name varchar(128) COMMENT '用户名',
    alias_name varchar(128) COMMENT '别名',
    status int COMMENT '集群状态',
    cloud_type int COMMENT '集群类型',
    kube_version varchar(128) COMMENT 'k8s 集群版本',
    node_number int COMMENT '集群节点数量',
    resources varchar(128) COMMENT '资源数量',
    description text COMMENT '描述',
    extension text COMMENT '扩展字段',
    KEY `idx_name` (`name`),
    UNIQUE KEY `name` (`name`)
) ENGINE=InnoDB CHARSET=utf8 AUTO_INCREMENT=22220801;
```

## 创建 `kube_configs` 表
```sql
CREATE TABLE `kube_configs` (
    id int primary key NOT NULL AUTO_INCREMENT COMMENT '主键' ,
    gmt_create datetime COMMENT '创建时间',
    gmt_modified datetime COMMENT '修改时间',
    resource_version int COMMENT '版本号',
    service_account varchar(128) COMMENT 'k8s service account',
    cloud_name varchar(128) COMMENT '集群名',
    cloud_id int COMMENT '所属 cloud id',
    cluster_role varchar(128) COMMENT 'k8s cluster role',
    config text COMMENT 'k8s kube_config',
    expiration_timestamp text COMMENT '过期时间',
    KEY `idx_cloud_name` (`cloud_name`),
    UNIQUE KEY `service_account` (`service_account`)
) ENGINE=InnoDB CHARSET=utf8 AUTO_INCREMENT=22220801;
```

## 创建 `nodes` 表
```sql
CREATE TABLE `nodes` (
    id int primary key NOT NULL AUTO_INCREMENT COMMENT '主键' ,
    gmt_create datetime COMMENT '创建时间',
    gmt_modified datetime COMMENT '修改时间',
    resource_version int COMMENT '版本号',
    cloud_id int COMMENT 'cloud ID',
    role varchar(128) COMMENT '节点类型',
    host_name varchar(128) COMMENT '节点名称',
    address varchar(128) COMMENT '节点 ip 地址',
    user varchar(128) COMMENT '用户名',
    password varchar(128) COMMENT '节点密码',
    KEY `idx_cloud` (`cloud_id`)
) ENGINE=InnoDB CHARSET=utf8 AUTO_INCREMENT=24220801;
```

## 创建 `events` 表
```sql
CREATE TABLE `events` (
    id int primary key NOT NULL AUTO_INCREMENT COMMENT '主键' ,
    gmt_create datetime COMMENT '创建时间',
    gmt_modified datetime COMMENT '修改时间',
    resource_version int COMMENT '版本号',
    user varchar(128) COMMENT '用户名称',
    client_ip varchar(128) COMMENT '登陆地址',
    operator varchar(128) COMMENT '操作类型',
    object varchar(128) COMMENT '操作对象',
    message varchar(128) COMMENT '消息'
) ENGINE=InnoDB CHARSET=utf8 AUTO_INCREMENT=26220801;
```

## 创建 `audit`
```sql
CREATE TABLE `audits` (
  `id` int primary key NOT NULL AUTO_INCREMENT COMMENT '主键' ,
  `gmt_create` datetime COMMENT '创建时间',
  `gmt_modified` datetime COMMENT '修改时间',
  `resource_version` int COMMENT '版本号',
  `operator` varchar(255) COLLATE utf8mb4_bin NOT NULL COMMENT '操作人',
  `action` varchar(255) COLLATE utf8mb4_bin NOT NULL COMMENT '动作',
  `ip` varchar(128) COLLATE utf8mb4_bin NOT NULL COMMENT '来源ip',
  `status` tinyint(4) COLLATE utf8mb4_bin NOT NULL COMMENT '执行是否成功：0-失败，1-成功',
  `path` varchar(255) COLLATE utf8mb4_bin NOT NULL COMMENT '详细内容',
  `resource_type` varchar(128) COLLATE utf8mb4_bin NOT NULL COMMENT '操作的资源类型'
) ENGINE=InnoDB AUTO_INCREMENT=3355 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin
```

## 创建 `cron_hpas` 表
```sql
CREATE TABLE `cron_hpas` (
  `id` bigint PRIMARY KEY NOT NULL AUTO_INCREMENT COMMENT '主键',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
  `resource_version` bigint NOT NULL DEFAULT 0 COMMENT '版本号',
  `name` varchar(128) NOT NULL COMMENT '规则名称',
  `cluster_name` varchar(128) NOT NULL COMMENT '集群名称',
  `namespace` varchar(128) NOT NULL COMMENT '命名空间',
  `target_kind` varchar(64) NOT NULL COMMENT '目标类型：Deployment/StatefulSet/HorizontalPodAutoscaler',
  `target_name` varchar(128) NOT NULL COMMENT '目标名称',
  `jobs` text COMMENT '定时任务列表（JSON 数组）',
  `exclude_dates` text COMMENT '排除日期规则集合（JSON 数组，可选）',
  `status` varchar(16) NOT NULL COMMENT '启停状态：active/paused',
  `description` varchar(255) DEFAULT NULL COMMENT '描述',
  `create_user` varchar(128) DEFAULT NULL COMMENT '创建人',
  KEY `idx_cluster_name` (`cluster_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='定时自动扩缩容规则';
```

## 创建 `cron_hpa_histories` 表
```sql
CREATE TABLE `cron_hpa_histories` (
  `id` bigint PRIMARY KEY NOT NULL AUTO_INCREMENT COMMENT '主键',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
  `resource_version` bigint NOT NULL DEFAULT 0 COMMENT '版本号',
  `cron_hpa_id` bigint NOT NULL COMMENT '关联 cron_hpas.id',
  `job_name` varchar(128) DEFAULT NULL COMMENT '定时任务名称',
  `scheduled_time` datetime DEFAULT NULL COMMENT '计划时间',
  `executed_at` datetime DEFAULT NULL COMMENT '实际执行时间',
  `previous_replicas` int NOT NULL DEFAULT 0 COMMENT '变更前副本数',
  `desired_replicas` int NOT NULL DEFAULT 0 COMMENT '目标副本数',
  `result` varchar(16) NOT NULL COMMENT '执行结果：Succeed/Failed/Skipped',
  `message` varchar(512) DEFAULT NULL COMMENT '执行信息',
  KEY `idx_cron_hpa_id` (`cron_hpa_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='定时自动扩缩容执行历史';
```
