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