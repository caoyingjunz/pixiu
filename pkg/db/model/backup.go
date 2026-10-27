package model

import (
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db/model/pixiu"
)

func init() {
	register(&BackupPlan{}, &Backup{}, &RestoreTask{})
}

type BackupStatus string

const (
	BackupStatusNew        BackupStatus = "New"
	BackupStatusInProgress BackupStatus = "InProgress"
	BackupStatusCompleted  BackupStatus = "Completed"
	BackupStatusFailed     BackupStatus = "Failed"
	BackupStatusDeleting   BackupStatus = "Deleting"
	BackupStatusUnknown    BackupStatus = "Unknown"
)

type RestoreStatus string

const (
	RestoreStatusNew        RestoreStatus = "New"
	RestoreStatusInProgress RestoreStatus = "InProgress"
	RestoreStatusCompleted  RestoreStatus = "Completed"
	RestoreStatusFailed     RestoreStatus = "Failed"
	RestoreStatusUnknown    RestoreStatus = "Unknown"
)

// BackupPlan describes a reusable Velero backup policy. Datasource-backed plans keep only
// the datasource reference; StorageConfig is retained for the legacy manual configuration path.
type BackupPlan struct {
	pixiu.Model

	Name                        string `gorm:"type:varchar(128);not null;index:idx_backup_plans_name" json:"name"`
	Description                 string `gorm:"type:text" json:"description"`
	UserId                      int64  `gorm:"not null;index:idx_backup_plans_user_id" json:"user_id"`
	SourceClusterId             int64  `gorm:"not null;index:idx_backup_plans_source_cluster_id" json:"source_cluster_id"`
	StorageDatasourceId         int64  `gorm:"default:0;index:idx_backup_plans_storage_datasource_id" json:"storage_datasource_id"`
	StorageBucket               string `gorm:"type:varchar(256)" json:"storage_bucket"`
	StorageLocation             string `gorm:"type:varchar(128);not null" json:"storage_location"`
	StorageConfig               string `gorm:"type:text" json:"storage_config"`
	VeleroNamespace             string `gorm:"type:varchar(128);not null;default:'velero'" json:"velero_namespace"`
	Schedule                    string `gorm:"type:varchar(128)" json:"schedule"`
	RetentionDays               int    `gorm:"default:30" json:"retention_days"`
	Namespaces                  string `gorm:"type:text" json:"namespaces"`
	IncludedResources           string `gorm:"type:text" json:"included_resources"`
	LabelSelector               string `gorm:"type:text" json:"label_selector"`
	IncludeClusterResources     bool   `gorm:"default:false" json:"include_cluster_resources"`
	SnapshotVolumes             bool   `gorm:"default:true" json:"snapshot_volumes"`
	Enabled                     bool   `gorm:"default:true" json:"enabled"`
	RestoreTargetClusterId      int64  `gorm:"default:0" json:"restore_target_cluster_id"`
	RestoreNamespace            string `gorm:"type:varchar(128)" json:"restore_namespace"`
	RestoreExistingResourceMode string `gorm:"type:varchar(16);default:'none'" json:"restore_existing_resource_mode"`
	RestoreVolumes              bool   `gorm:"default:true" json:"restore_volumes"`
}

func (BackupPlan) TableName() string { return "backup_plans" }

// Backup is Pixiu's record of a Velero Backup CR created in its source cluster.
type Backup struct {
	pixiu.Model

	PlanId              int64        `gorm:"index:idx_backups_plan_id" json:"plan_id"`
	UserId              int64        `gorm:"not null;index:idx_backups_user_id" json:"user_id"`
	SourceClusterId     int64        `gorm:"not null;index:idx_backups_source_cluster_id" json:"source_cluster_id"`
	StorageDatasourceId int64        `gorm:"default:0;index:idx_backups_storage_datasource_id" json:"storage_datasource_id"`
	StorageBucket       string       `gorm:"type:varchar(256)" json:"storage_bucket"`
	StorageConfig       string       `gorm:"type:text" json:"storage_config"`
	StorageLocation     string       `gorm:"type:varchar(128);not null" json:"storage_location"`
	VeleroNamespace     string       `gorm:"type:varchar(128);not null;default:'velero'" json:"velero_namespace"`
	ExternalName        string       `gorm:"type:varchar(253);not null;uniqueIndex:idx_backups_external" json:"external_name"`
	Status              BackupStatus `gorm:"type:varchar(32);not null;index:idx_backups_status" json:"status"`
	StartedAt           *time.Time   `gorm:"type:datetime" json:"started_at"`
	CompletedAt         *time.Time   `gorm:"type:datetime" json:"completed_at"`
	ExpiresAt           *time.Time   `gorm:"type:datetime" json:"expires_at"`
	Warnings            string       `gorm:"type:text" json:"warnings"`
	ErrorMessage        string       `gorm:"type:text" json:"error_message"`
}

func (Backup) TableName() string { return "backups" }

// RestoreTask keeps the source and target cluster explicit to support A-to-B and B-to-A recovery.
type RestoreTask struct {
	pixiu.Model

	Name                 string        `gorm:"type:varchar(128);not null;default:'';index:idx_restore_tasks_name" json:"name"`
	Description          string        `gorm:"type:text" json:"description"`
	UserId               int64         `gorm:"not null;index:idx_restore_tasks_user_id" json:"user_id"`
	BackupId             int64         `gorm:"not null;index:idx_restore_tasks_backup_id" json:"backup_id"`
	SourceClusterId      int64         `gorm:"not null;index:idx_restore_tasks_source_cluster_id" json:"source_cluster_id"`
	TargetClusterId      int64         `gorm:"not null;index:idx_restore_tasks_target_cluster_id" json:"target_cluster_id"`
	VeleroNamespace      string        `gorm:"type:varchar(128);not null;default:'velero'" json:"velero_namespace"`
	ExternalName         string        `gorm:"type:varchar(253);not null;uniqueIndex:idx_restore_tasks_external" json:"external_name"`
	IncludedNamespaces   string        `gorm:"type:text" json:"included_namespaces"`
	ExcludedNamespaces   string        `gorm:"type:text" json:"excluded_namespaces"`
	NamespaceMapping     string        `gorm:"type:text" json:"namespace_mapping"`
	StorageClassMapping  string        `gorm:"type:text" json:"storage_class_mapping"`
	ExistingResourceMode string        `gorm:"type:varchar(16);not null;default:'skip'" json:"existing_resource_mode"`
	RestoreVolumes       bool          `gorm:"default:true" json:"restore_volumes"`
	Status               RestoreStatus `gorm:"type:varchar(32);not null;index:idx_restore_tasks_status" json:"status"`
	StartedAt            *time.Time    `gorm:"type:datetime" json:"started_at"`
	CompletedAt          *time.Time    `gorm:"type:datetime" json:"completed_at"`
	Warnings             string        `gorm:"type:text" json:"warnings"`
	ErrorMessage         string        `gorm:"type:text" json:"error_message"`
}

func (RestoreTask) TableName() string { return "restore_tasks" }
