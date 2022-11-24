# 创建 `gopixiu` 数据库
```sql
CREATE DATABASE gopixiu;
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

### 创建 `gopixiu` 用户
```sql
# 用户 gopixiu 的初始密码为 gopixiu
insert into users(name, password) values ('gopixiu', '$2a$10$KsAmVnOI7lzOZwyC8A9bvujTcLqsR7p01qgPmT1cpN6V7Au6OtAKC');
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

## 创建 `clusters` 表
```sql
CREATE TABLE `clusters` (
    id int primary key NOT NULL AUTO_INCREMENT COMMENT '主键' ,
    gmt_create datetime COMMENT '创建时间',
    gmt_modified datetime COMMENT '修改时间',
    resource_version int COMMENT '版本号',
    cloud_id int COMMENT 'cloud ID',
    api_server varchar(128) COMMENT 'k8s apiServer 地址',
    version varchar(128) COMMENT 'k8s 集群版本',
    runtime varchar(128) COMMENT '容器运行时',
    cni varchar(128) COMMENT '集群 cni',
    service_cidr varchar(128) COMMENT 'service 网段',
    pod_cidr varchar(128) COMMENT 'pod 网段',
    proxy_mode varchar(128) COMMENT 'kubeProxy 模式',
    KEY `idx_cloud` (`cloud_id`),
    UNIQUE KEY `cloud_id` (`cloud_id`)
) ENGINE=InnoDB CHARSET=utf8 AUTO_INCREMENT=23220801;
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
