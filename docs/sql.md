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

### 创建 `admin` 用户
```sql
insert into users(name, password) values ('admin', 'a66abb5684c45962d887564f08346e8d');
```

## 创建 `clouds` 表
```sql
CREATE TABLE `clouds` (
    id int primary key NOT NULL AUTO_INCREMENT COMMENT '主键' ,
    gmt_create datetime COMMENT '创建时间',
    gmt_modified datetime COMMENT '修改时间',
    resource_version int COMMENT '版本号',
    name varchar(128) COMMENT '用户名',
    status int COMMENT '集群状态',
    cloud_type varchar(128) COMMENT '集群类型',
    kube_version varchar(128) COMMENT 'k8s 集群版本',
    kube_config text COMMENT 'kubeConfig',
    node_number int COMMENT '集群节点数量',
    resources varchar(128) COMMENT '资源数量',
    description text COMMENT '描述',
    extension text COMMENT '扩展字段',
    KEY `idx_name` (`name`),
    UNIQUE KEY `name` (`name`)
) ENGINE=InnoDB CHARSET=utf8 AUTO_INCREMENT=22220801;
```
