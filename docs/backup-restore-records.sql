-- 备份记录表重命名 + 恢复任务/执行记录拆分 —— 存量库手工运维 SQL
--
-- 全新数据库无需执行本文件：AutoMigrate 会直接创建 backup_plans / backup_records /
-- restore_tasks / restore_records。
--
-- 部署顺序：先在存量库执行本 SQL，再启动新版本服务（AutoMigrate 会创建 restore_records）。
-- 历史恢复任务的执行状态按要求不迁移：执行态字段直接删除，执行记录从空表开始。

-- 1) 备份记录表重命名（已有数据保留）
RENAME TABLE backups TO backup_records;

-- 2) 恢复任务去掉执行态字段（历史执行状态不迁移，按产品要求一步到位）
ALTER TABLE restore_tasks DROP INDEX idx_restore_tasks_external;
ALTER TABLE restore_tasks DROP INDEX idx_restore_tasks_status;
ALTER TABLE restore_tasks
  DROP COLUMN external_name,
  DROP COLUMN status,
  DROP COLUMN started_at,
  DROP COLUMN completed_at,
  DROP COLUMN warnings,
  DROP COLUMN error_message;
