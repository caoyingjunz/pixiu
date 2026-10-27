package types

import (
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

type BackupPlan struct {
	PixiuMeta                   `json:",inline"`
	TimeMeta                    `json:",inline"`
	Name                        string              `json:"name"`
	Description                 string              `json:"description"`
	UserId                      int64               `json:"user_id"`
	SourceClusterId             int64               `json:"source_cluster_id"`
	StorageDatasourceId         int64               `json:"storage_datasource_id"`
	StorageBucket               string              `json:"storage_bucket"`
	StorageLocation             string              `json:"storage_location"`
	StorageConfig               BackupStorageConfig `json:"storage_config"`
	VeleroNamespace             string              `json:"velero_namespace"`
	Schedule                    string              `json:"schedule"`
	RetentionDays               int                 `json:"retention_days"`
	Namespaces                  []string            `json:"namespaces"`
	IncludedResources           []string            `json:"included_resources"`
	LabelSelector               string              `json:"label_selector"`
	IncludeClusterResources     bool                `json:"include_cluster_resources"`
	SnapshotVolumes             bool                `json:"snapshot_volumes"`
	Enabled                     bool                `json:"enabled"`
	RestoreTargetClusterId      int64               `json:"restore_target_cluster_id,omitempty"`
	RestoreNamespace            string              `json:"restore_namespace,omitempty"`
	RestoreExistingResourceMode string              `json:"restore_existing_resource_mode,omitempty"`
	RestoreVolumes              bool                `json:"restore_volumes"`
}

type BackupStorageConfig struct {
	Provider              string `json:"provider"`
	Endpoint              string `json:"endpoint"`
	Bucket                string `json:"bucket"`
	Prefix                string `json:"prefix"`
	Region                string `json:"region"`
	AccessKey             string `json:"access_key"`
	SecretKey             string `json:"secret_key"`
	SessionToken          string `json:"session_token"`
	UsePathStyle          bool   `json:"use_path_style"`
	InsecureSkipTLSVerify bool   `json:"insecure_skip_tls_verify"`
}

type VeleroStatus struct {
	Installed  bool     `json:"installed"`
	Namespaces []string `json:"namespaces"`
}

type Backup struct {
	PixiuMeta           `json:",inline"`
	TimeMeta            `json:",inline"`
	PlanId              int64              `json:"plan_id"`
	SourceClusterId     int64              `json:"source_cluster_id"`
	StorageDatasourceId int64              `json:"storage_datasource_id"`
	StorageBucket       string             `json:"storage_bucket"`
	StorageLocation     string             `json:"storage_location"`
	VeleroNamespace     string             `json:"velero_namespace"`
	ExternalName        string             `json:"external_name"`
	Status              model.BackupStatus `json:"status"`
	StartedAt           *time.Time         `json:"started_at"`
	CompletedAt         *time.Time         `json:"completed_at"`
	ExpiresAt           *time.Time         `json:"expires_at"`
	Warnings            string             `json:"warnings"`
	ErrorMessage        string             `json:"error_message"`
}

type RestoreTask struct {
	PixiuMeta            `json:",inline"`
	TimeMeta             `json:",inline"`
	Name                 string              `json:"name"`
	Description          string              `json:"description"`
	BackupId             int64               `json:"backup_id"`
	SourceClusterId      int64               `json:"source_cluster_id"`
	TargetClusterId      int64               `json:"target_cluster_id"`
	VeleroNamespace      string              `json:"velero_namespace"`
	ExternalName         string              `json:"external_name"`
	IncludedNamespaces   []string            `json:"included_namespaces"`
	ExcludedNamespaces   []string            `json:"excluded_namespaces"`
	NamespaceMapping     map[string]string   `json:"namespace_mapping"`
	StorageClassMapping  map[string]string   `json:"storage_class_mapping"`
	ExistingResourceMode string              `json:"existing_resource_mode"`
	RestoreVolumes       bool                `json:"restore_volumes"`
	Status               model.RestoreStatus `json:"status"`
	StartedAt            *time.Time          `json:"started_at"`
	CompletedAt          *time.Time          `json:"completed_at"`
	Warnings             string              `json:"warnings"`
	ErrorMessage         string              `json:"error_message"`
}

type CreateBackupPlanRequest struct {
	Name                        string              `json:"name" binding:"required,max=50"`
	Description                 string              `json:"description"`
	SourceClusterId             int64               `json:"source_cluster_id" binding:"required"`
	StorageDatasourceId         int64               `json:"storage_datasource_id"`
	StorageBucket               string              `json:"storage_bucket"`
	StorageLocation             string              `json:"storage_location" binding:"max=128"`
	StorageConfig               BackupStorageConfig `json:"storage_config"`
	VeleroNamespace             string              `json:"velero_namespace"`
	Schedule                    string              `json:"schedule"`
	RetentionDays               int                 `json:"retention_days"`
	Namespaces                  []string            `json:"namespaces"`
	IncludedResources           []string            `json:"included_resources"`
	LabelSelector               string              `json:"label_selector"`
	IncludeClusterResources     bool                `json:"include_cluster_resources"`
	SnapshotVolumes             *bool               `json:"snapshot_volumes"`
	Enabled                     *bool               `json:"enabled"`
	RestoreTargetClusterId      int64               `json:"restore_target_cluster_id"`
	RestoreNamespace            string              `json:"restore_namespace"`
	RestoreExistingResourceMode string              `json:"restore_existing_resource_mode"`
	RestoreVolumes              *bool               `json:"restore_volumes"`
	UserId                      int64               `json:"-"`
}

func (r *CreateBackupPlanRequest) SetUserID(id int64) { r.UserId = id }

type UpdateBackupPlanRequest struct {
	// Use a pointer so the initial revision (0) is accepted while an omitted value is rejected.
	ResourceVersion             *int64               `json:"resource_version" binding:"required"`
	Name                        *string              `json:"name" binding:"omitempty,max=50"`
	Description                 *string              `json:"description"`
	SourceClusterId             *int64               `json:"source_cluster_id"`
	StorageDatasourceId         *int64               `json:"storage_datasource_id"`
	StorageBucket               *string              `json:"storage_bucket"`
	StorageLocation             *string              `json:"storage_location"`
	StorageConfig               *BackupStorageConfig `json:"storage_config"`
	Schedule                    *string              `json:"schedule"`
	RetentionDays               *int                 `json:"retention_days"`
	Namespaces                  *[]string            `json:"namespaces"`
	IncludedResources           *[]string            `json:"included_resources"`
	LabelSelector               *string              `json:"label_selector"`
	IncludeClusterResources     *bool                `json:"include_cluster_resources"`
	SnapshotVolumes             *bool                `json:"snapshot_volumes"`
	Enabled                     *bool                `json:"enabled"`
	RestoreTargetClusterId      *int64               `json:"restore_target_cluster_id"`
	RestoreNamespace            *string              `json:"restore_namespace"`
	RestoreExistingResourceMode *string              `json:"restore_existing_resource_mode"`
	RestoreVolumes              *bool                `json:"restore_volumes"`
}

type RunBackupPlanRequest struct {
	Name string `json:"name"`
}

type CreateRestoreRequest struct {
	Name                 string            `json:"name" binding:"required,max=50"`
	Description          string            `json:"description"`
	BackupId             int64             `json:"backup_id" binding:"required"`
	TargetClusterId      int64             `json:"target_cluster_id" binding:"required"`
	VeleroNamespace      string            `json:"velero_namespace"`
	IncludedNamespaces   []string          `json:"included_namespaces"`
	ExcludedNamespaces   []string          `json:"excluded_namespaces"`
	NamespaceMapping     map[string]string `json:"namespace_mapping"`
	StorageClassMapping  map[string]string `json:"storage_class_mapping"`
	ExistingResourceMode string            `json:"existing_resource_mode"`
	RestoreVolumes       *bool             `json:"restore_volumes"`
}

type UpdateRestoreRequest struct {
	Name string `json:"name" binding:"required,max=50"`
}
