# 常见问题汇总（FAQ）

> 最后更新：2026-10-07　|　维护：平台组

每个问题对应一个独立 Markdown 文档，本文件仅作索引。

## 索引

| 编号 | 标题 | 分类 | 文档 |
|---|---|---|---|
| FAQ-001 | 监控页 etcd 指标为空 | 监控与数据源 | [etcd-metrics-empty.md](./etcd-metrics-empty.md) |
| FAQ-002 | 无法通过「导入集群」管理 AWS EKS | 容器服务 / 集群接入 | [eks-import-cluster.md](./eks-import-cluster.md) |
| FAQ-003 | 监控页无数据（未关联 Prometheus） | 监控与数据源 | [monitor-no-data.md](./monitor-no-data.md) |
| FAQ-004 | runner 任务容器日志查看（nerdctl logs 报错） | 部署与运行时 | [runner-container-logs.md](./runner-container-logs.md) |
| FAQ-005 | 数据源密码加密与密钥轮换 | 数据源与安全 | [datasource-password-encryption.md](./datasource-password-encryption.md) |

## 新增 FAQ 规范

1. 新建独立文件，命名：`<关键词>-<关键词>.md`（如 `etcd-metrics-empty.md`）
2. 文档结构固定：**现象 / 根因 / 验证方式 / 解决方案 / 备注**
3. 在本索引表追加一行并更新「最后更新」日期
