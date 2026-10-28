package backup

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/caoyingjunz/pixiu/cmd/app/config"
	clustercontroller "github.com/caoyingjunz/pixiu/pkg/controller/cluster"
	storagecontroller "github.com/caoyingjunz/pixiu/pkg/controller/extension/storage"
	controllerutil "github.com/caoyingjunz/pixiu/pkg/controller/util"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type Getter interface{ Backup() Interface }
type Interface interface {
	GetVeleroStatus(context.Context, int64) (*types.VeleroStatus, error)
	CreatePlan(context.Context, *types.CreateBackupPlanRequest) (*types.BackupPlan, error)
	UpdatePlan(context.Context, int64, *types.UpdateBackupPlanRequest) error
	DeletePlan(context.Context, int64) error
	ListPlans(context.Context) ([]types.BackupPlan, error)
	GetPlan(context.Context, int64) (*types.BackupPlan, error)
	RunPlan(context.Context, int64, *types.RunBackupPlanRequest) (*types.Backup, error)
	ListBackups(context.Context) ([]types.Backup, error)
	GetBackup(context.Context, int64) (*types.Backup, error)
	DeleteBackup(context.Context, int64) error
	CreateRestore(context.Context, *types.CreateRestoreRequest) (*types.RestoreTask, error)
	UpdateRestore(context.Context, int64, *types.UpdateRestoreRequest) error
	DeleteRestore(context.Context, int64) error
	ListRestores(context.Context) ([]types.RestoreTask, error)
	GetRestore(context.Context, int64) (*types.RestoreTask, error)
}

type controller struct {
	cc       config.Config
	factory  db.ShareDaoFactory
	clusters clustercontroller.Interface
	storage  storagecontroller.Interface
}

func New(cc config.Config, factory db.ShareDaoFactory, clusters clustercontroller.Interface, storage storagecontroller.Interface) Interface {
	return &controller{cc: cc, factory: factory, clusters: clusters, storage: storage}
}

func (c *controller) GetVeleroStatus(ctx context.Context, clusterID int64) (*types.VeleroStatus, error) {
	if err := c.ensureClusterAccess(ctx, clusterID); err != nil {
		return nil, err
	}
	client, err := c.velero(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	namespaces, err := client.AvailableNamespaces(ctx)
	if err != nil {
		return nil, err
	}
	return &types.VeleroStatus{Installed: len(namespaces) > 0, Namespaces: namespaces}, nil
}

func (c *controller) CreatePlan(ctx context.Context, req *types.CreateBackupPlanRequest) (*types.BackupPlan, error) {
	if err := c.ensureClusterAccess(ctx, req.SourceClusterId); err != nil {
		return nil, err
	}
	config := storageConfigToEngine(req.StorageConfig)
	if req.StorageDatasourceId > 0 {
		var resolveErr error
		config, req.StorageBucket, req.StorageLocation, resolveErr = c.storageConfigFromDatasource(ctx, req.StorageDatasourceId, req.StorageBucket)
		if resolveErr != nil {
			return nil, resolveErr
		}
	}
	client, err := c.velero(ctx, req.SourceClusterId)
	if err != nil {
		return nil, err
	}
	namespace, err := resolveVeleroNamespace(ctx, client, req.VeleroNamespace)
	if err != nil {
		return nil, err
	}
	// Draft plans may omit a storage location; Velero validates it when a backup runs.
	storageConfig := ""
	if req.StorageDatasourceId == 0 {
		raw, _ := json.Marshal(req.StorageConfig)
		storageConfig = string(raw)
	}
	if strings.TrimSpace(req.StorageLocation) != "" {
		if err = client.EnsureConfiguredStorageLocation(ctx, namespace, req.StorageLocation, &config); err != nil {
			return nil, err
		}
	}
	snapshotVolumes, enabled := true, true
	if req.SnapshotVolumes != nil {
		snapshotVolumes = *req.SnapshotVolumes
	}
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	retention := req.RetentionDays
	if retention <= 0 {
		retention = 30
	}
	namespaces, _ := json.Marshal(req.Namespaces)
	includedResources, _ := json.Marshal(req.IncludedResources)
	plan, err := c.factory.Backup().Plan().Create(ctx, &model.BackupPlan{Name: req.Name, Description: req.Description, UserId: req.UserId, SourceClusterId: req.SourceClusterId, StorageDatasourceId: req.StorageDatasourceId, StorageBucket: req.StorageBucket, StorageLocation: req.StorageLocation, StorageConfig: storageConfig, VeleroNamespace: namespace, Schedule: strings.TrimSpace(req.Schedule), RetentionDays: retention, Namespaces: string(namespaces), IncludedResources: string(includedResources), LabelSelector: req.LabelSelector, IncludeClusterResources: req.IncludeClusterResources, SnapshotVolumes: snapshotVolumes, RestoreTargetClusterId: req.RestoreTargetClusterId, RestoreNamespace: req.RestoreNamespace, RestoreExistingResourceMode: req.RestoreExistingResourceMode, RestoreVolumes: restoreVolumesValue(req.RestoreVolumes), Enabled: enabled})
	if err != nil {
		return nil, err
	}
	if plan.Enabled && plan.Schedule != "" {
		_, err = client.CreateSchedule(ctx, namespace, planExternalName(plan.Id), plan.Schedule, c.planSpec(plan))
		if err != nil {
			_ = c.factory.Backup().Plan().Delete(ctx, plan.Id)
			return nil, err
		}
	}
	return toPlan(plan), nil
}

func (c *controller) UpdatePlan(ctx context.Context, id int64, req *types.UpdateBackupPlanRequest) error {
	plan, err := c.plan(ctx, id)
	if err != nil {
		return err
	}
	if err = c.ensureClusterAccess(ctx, plan.SourceClusterId); err != nil {
		return err
	}
	updates := map[string]interface{}{}
	if req.SourceClusterId != nil && *req.SourceClusterId != plan.SourceClusterId {
		if err = c.ensureClusterAccess(ctx, *req.SourceClusterId); err != nil {
			return err
		}
		updates["source_cluster_id"] = *req.SourceClusterId
	}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.StorageDatasourceId != nil {
		updates["storage_datasource_id"] = *req.StorageDatasourceId
		if *req.StorageDatasourceId > 0 {
			bucket := plan.StorageBucket
			if req.StorageBucket != nil {
				bucket = *req.StorageBucket
			}
			_, bucket, location, resolveErr := c.storageConfigFromDatasource(ctx, *req.StorageDatasourceId, bucket)
			if resolveErr != nil {
				return resolveErr
			}
			updates["storage_bucket"] = bucket
			updates["storage_config"] = ""
			updates["storage_location"] = location
		}
	}
	if req.StorageBucket != nil && req.StorageDatasourceId == nil && plan.StorageDatasourceId > 0 {
		_, bucket, location, resolveErr := c.storageConfigFromDatasource(ctx, plan.StorageDatasourceId, *req.StorageBucket)
		if resolveErr != nil {
			return resolveErr
		}
		updates["storage_bucket"] = bucket
		updates["storage_location"] = location
	}
	if req.StorageLocation != nil && (req.StorageDatasourceId == nil || *req.StorageDatasourceId == 0) {
		updates["storage_location"] = *req.StorageLocation
	}
	if req.StorageBucket != nil && req.StorageDatasourceId == nil && plan.StorageDatasourceId == 0 {
		updates["storage_bucket"] = *req.StorageBucket
		var config types.BackupStorageConfig
		_ = json.Unmarshal([]byte(plan.StorageConfig), &config)
		config.Bucket = *req.StorageBucket
		raw, _ := json.Marshal(config)
		updates["storage_config"] = string(raw)
	}
	if req.StorageConfig != nil && (req.StorageDatasourceId == nil || *req.StorageDatasourceId == 0) {
		config := *req.StorageConfig
		var previous types.BackupStorageConfig
		_ = json.Unmarshal([]byte(plan.StorageConfig), &previous)
		if strings.TrimSpace(config.SecretKey) == "" {
			config.SecretKey = previous.SecretKey
		}
		if strings.TrimSpace(config.AccessKey) == "" {
			config.AccessKey = previous.AccessKey
		}
		raw, _ := json.Marshal(config)
		updates["storage_config"] = string(raw)
	}
	if req.Schedule != nil {
		updates["schedule"] = *req.Schedule
	}
	if req.RetentionDays != nil {
		updates["retention_days"] = *req.RetentionDays
	}
	if req.LabelSelector != nil {
		updates["label_selector"] = *req.LabelSelector
	}
	if req.IncludeClusterResources != nil {
		updates["include_cluster_resources"] = *req.IncludeClusterResources
	}
	if req.SnapshotVolumes != nil {
		updates["snapshot_volumes"] = *req.SnapshotVolumes
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.RestoreTargetClusterId != nil {
		updates["restore_target_cluster_id"] = *req.RestoreTargetClusterId
	}
	if req.RestoreNamespace != nil {
		updates["restore_namespace"] = *req.RestoreNamespace
	}
	if req.RestoreExistingResourceMode != nil {
		updates["restore_existing_resource_mode"] = *req.RestoreExistingResourceMode
	}
	if req.RestoreVolumes != nil {
		updates["restore_volumes"] = *req.RestoreVolumes
	}
	if req.Namespaces != nil {
		raw, _ := json.Marshal(*req.Namespaces)
		updates["namespaces"] = string(raw)
	}
	if req.IncludedResources != nil {
		raw, _ := json.Marshal(*req.IncludedResources)
		updates["included_resources"] = string(raw)
	}
	updated := *plan
	applyPlanUpdates(&updated, updates)
	config, err := c.planStorageConfig(ctx, &updated)
	if err != nil {
		return err
	}
	client, err := c.velero(ctx, updated.SourceClusterId)
	if err != nil {
		return err
	}
	if updated.SourceClusterId != plan.SourceClusterId {
		namespace, resolveErr := resolveVeleroNamespace(ctx, client, "")
		if resolveErr != nil {
			return fmt.Errorf("resolve Velero namespace for cluster %d: %w", updated.SourceClusterId, resolveErr)
		}
		updates["velero_namespace"] = namespace
		updated.VeleroNamespace = namespace
	}
	if _, err = resolveVeleroNamespace(ctx, client, updated.VeleroNamespace); err != nil {
		return err
	}
	if strings.TrimSpace(updated.StorageLocation) != "" {
		if err = client.EnsureConfiguredStorageLocation(ctx, updated.VeleroNamespace, updated.StorageLocation, &config); err != nil {
			return err
		}
	}
	if updated.Enabled && updated.Schedule != "" {
		if _, err = client.UpsertSchedule(ctx, updated.VeleroNamespace, planExternalName(updated.Id), updated.Schedule, c.planSpec(&updated)); err != nil {
			return err
		}
		if updated.SourceClusterId != plan.SourceClusterId || updated.VeleroNamespace != plan.VeleroNamespace {
			previousClient := client
			if updated.SourceClusterId != plan.SourceClusterId {
				previousClient, err = c.velero(ctx, plan.SourceClusterId)
				if err != nil {
					return err
				}
			}
			if err = previousClient.DeleteSchedule(ctx, plan.VeleroNamespace, planExternalName(plan.Id)); err != nil && !strings.Contains(err.Error(), "not found") {
				return err
			}
		}
	} else {
		previousClient := client
		if updated.SourceClusterId != plan.SourceClusterId {
			previousClient, err = c.velero(ctx, plan.SourceClusterId)
			if err != nil {
				return err
			}
		}
		err = previousClient.DeleteSchedule(ctx, plan.VeleroNamespace, planExternalName(plan.Id))
		// A deleted schedule is equivalent to the desired disabled state.
		if err != nil && !strings.Contains(err.Error(), "not found") {
			return err
		}
	}
	return c.factory.Backup().Plan().Update(ctx, id, *req.ResourceVersion, updates)
}

func (c *controller) DeletePlan(ctx context.Context, id int64) error {
	plan, err := c.plan(ctx, id)
	if err != nil {
		return err
	}
	if err = c.ensureClusterAccess(ctx, plan.SourceClusterId); err != nil {
		return err
	}
	client, err := c.velero(ctx, plan.SourceClusterId)
	if err != nil {
		return err
	}
	if err = client.DeleteSchedule(ctx, plan.VeleroNamespace, planExternalName(plan.Id)); err != nil && !strings.Contains(err.Error(), "not found") {
		return err
	}
	return c.factory.Backup().Plan().Delete(ctx, id)
}
func (c *controller) ListPlans(ctx context.Context) ([]types.BackupPlan, error) {
	objects, err := c.factory.Backup().Plan().List(ctx, db.WithModifyOrderByDesc())
	if err != nil {
		return nil, err
	}
	result := make([]types.BackupPlan, 0, len(objects))
	for i := range objects {
		if controllerutil.CheckResourceOwner(ctx, objects[i].UserId) == nil && c.ensureClusterAccess(ctx, objects[i].SourceClusterId) == nil {
			result = append(result, *toPlan(&objects[i]))
		}
	}
	return result, nil
}

func (c *controller) GetPlan(ctx context.Context, id int64) (*types.BackupPlan, error) {
	plan, err := c.plan(ctx, id)
	if err != nil {
		return nil, err
	}
	if err = c.ensureClusterAccess(ctx, plan.SourceClusterId); err != nil {
		return nil, err
	}
	return toPlan(plan), nil
}

func (c *controller) RunPlan(ctx context.Context, id int64, req *types.RunBackupPlanRequest) (*types.Backup, error) {
	plan, err := c.plan(ctx, id)
	if err != nil {
		return nil, err
	}
	if !plan.Enabled {
		return nil, fmt.Errorf("backup plan is disabled")
	}
	if err = c.ensureClusterAccess(ctx, plan.SourceClusterId); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = fmt.Sprintf("pixiu-backup-%d", time.Now().UnixNano())
	}
	record, err := c.factory.Backup().Record().Create(ctx, &model.Backup{PlanId: plan.Id, UserId: plan.UserId, SourceClusterId: plan.SourceClusterId, StorageDatasourceId: plan.StorageDatasourceId, StorageBucket: plan.StorageBucket, StorageConfig: plan.StorageConfig, StorageLocation: plan.StorageLocation, VeleroNamespace: plan.VeleroNamespace, ExternalName: name, Status: model.BackupStatusNew})
	if err != nil {
		return nil, err
	}
	client, err := c.velero(ctx, plan.SourceClusterId)
	if err == nil {
		_, err = resolveVeleroNamespace(ctx, client, plan.VeleroNamespace)
	}
	if err == nil {
		config, configErr := c.planStorageConfig(ctx, plan)
		if configErr != nil {
			err = configErr
		} else {
			err = client.EnsureConfiguredStorageLocation(ctx, plan.VeleroNamespace, plan.StorageLocation, &config)
		}
	}
	if err == nil {
		_, err = client.CreateBackup(ctx, plan.VeleroNamespace, c.planSpec(plan, name))
	}
	if err != nil {
		_ = c.factory.Backup().Record().UpdateInternal(context.Background(), record.Id, map[string]interface{}{"status": model.BackupStatusFailed, "error_message": err.Error()})
		return nil, fmt.Errorf("create Velero backup: %w", err)
	}
	now := time.Now()
	_ = c.factory.Backup().Record().UpdateInternal(ctx, record.Id, map[string]interface{}{"status": model.BackupStatusInProgress, "started_at": now})
	record.Status, record.StartedAt = model.BackupStatusInProgress, &now
	return toBackup(record), nil
}

func (c *controller) ListBackups(ctx context.Context) ([]types.Backup, error) {
	objects, err := c.factory.Backup().Record().List(ctx, db.WithModifyOrderByDesc())
	if err != nil {
		return nil, err
	}
	result := make([]types.Backup, 0, len(objects))
	for i := range objects {
		if controllerutil.CheckResourceOwner(ctx, objects[i].UserId) == nil && c.ensureClusterAccess(ctx, objects[i].SourceClusterId) == nil {
			c.syncBackup(ctx, &objects[i])
			result = append(result, *toBackup(&objects[i]))
		}
	}
	return result, nil
}
func (c *controller) GetBackup(ctx context.Context, id int64) (*types.Backup, error) {
	object, err := c.backup(ctx, id)
	if err != nil {
		return nil, err
	}
	if err = c.ensureClusterAccess(ctx, object.SourceClusterId); err != nil {
		return nil, err
	}
	c.syncBackup(ctx, object)
	return toBackup(object), nil
}
func (c *controller) DeleteBackup(ctx context.Context, id int64) error {
	object, err := c.backup(ctx, id)
	if err != nil {
		return err
	}
	if err = c.ensureClusterAccess(ctx, object.SourceClusterId); err != nil {
		return err
	}
	client, err := c.velero(ctx, object.SourceClusterId)
	if err != nil {
		return err
	}
	if err = client.DeleteBackup(ctx, object.VeleroNamespace, object.ExternalName); err != nil {
		return err
	}
	return c.factory.Backup().Record().Delete(ctx, id)
}

func (c *controller) CreateRestore(ctx context.Context, req *types.CreateRestoreRequest) (*types.RestoreTask, error) {
	restoreName := strings.TrimSpace(req.Name)
	if restoreName == "" {
		return nil, fmt.Errorf("restore name is required")
	}
	backup, err := c.backup(ctx, req.BackupId)
	if err != nil {
		return nil, err
	}
	if err = c.ensureClusterAccess(ctx, backup.SourceClusterId); err != nil {
		return nil, err
	}
	if err = c.ensureClusterAccess(ctx, req.TargetClusterId); err != nil {
		return nil, err
	}
	if len(req.StorageClassMapping) > 0 {
		return nil, fmt.Errorf("StorageClass mapping requires Velero resource modifiers and is not enabled")
	}
	c.syncBackup(ctx, backup)
	if backup.Status != model.BackupStatusCompleted {
		return nil, fmt.Errorf("backup %q is not completed", backup.ExternalName)
	}
	client, err := c.velero(ctx, req.TargetClusterId)
	if err != nil {
		return nil, err
	}
	namespace, err := resolveVeleroNamespace(ctx, client, req.VeleroNamespace)
	if err != nil {
		return nil, err
	}
	config, err := c.backupStorageConfig(ctx, backup)
	if err != nil {
		return nil, err
	}
	if err = client.EnsureConfiguredStorageLocation(ctx, namespace, backup.StorageLocation, &config); err != nil {
		return nil, err
	}
	mode := normalizeExistingResourceMode(req.ExistingResourceMode)
	restoreVolumes := true
	if req.RestoreVolumes != nil {
		restoreVolumes = *req.RestoreVolumes
	}
	namespaceMapping, _ := json.Marshal(req.NamespaceMapping)
	includedNamespaces, _ := json.Marshal(req.IncludedNamespaces)
	excludedNamespaces, _ := json.Marshal(req.ExcludedNamespaces)
	storageClassMapping, _ := json.Marshal(req.StorageClassMapping)
	name := fmt.Sprintf("pixiu-restore-%d", time.Now().UnixNano())
	task, err := c.factory.Backup().Restore().Create(ctx, &model.RestoreTask{Name: restoreName, Description: strings.TrimSpace(req.Description), UserId: backup.UserId, BackupId: backup.Id, SourceClusterId: backup.SourceClusterId, TargetClusterId: req.TargetClusterId, VeleroNamespace: namespace, ExternalName: name, IncludedNamespaces: string(includedNamespaces), ExcludedNamespaces: string(excludedNamespaces), NamespaceMapping: string(namespaceMapping), StorageClassMapping: string(storageClassMapping), ExistingResourceMode: mode, RestoreVolumes: restoreVolumes, Status: model.RestoreStatusNew})
	if err != nil {
		return nil, err
	}
	_, err = client.CreateRestore(ctx, namespace, RestoreSpec{Name: name, BackupName: backup.ExternalName, IncludedNamespaces: req.IncludedNamespaces, ExcludedNamespaces: req.ExcludedNamespaces, NamespaceMapping: req.NamespaceMapping, ExistingResourceMode: mode, RestoreVolumes: restoreVolumes})
	if err != nil {
		_ = c.factory.Backup().Restore().UpdateInternal(context.Background(), task.Id, map[string]interface{}{"status": model.RestoreStatusFailed, "error_message": err.Error()})
		return nil, fmt.Errorf("create Velero restore: %w", err)
	}
	now := time.Now()
	_ = c.factory.Backup().Restore().UpdateInternal(ctx, task.Id, map[string]interface{}{"status": model.RestoreStatusInProgress, "started_at": now})
	task.Status, task.StartedAt = model.RestoreStatusInProgress, &now
	return toRestore(task), nil
}

func (c *controller) UpdateRestore(ctx context.Context, id int64, req *types.UpdateRestoreRequest) error {
	task, err := c.restore(ctx, id)
	if err != nil {
		return err
	}
	if err = c.ensureClusterAccess(ctx, task.SourceClusterId); err != nil {
		return err
	}
	if err = c.ensureClusterAccess(ctx, task.TargetClusterId); err != nil {
		return err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return fmt.Errorf("restore name is required")
	}
	return c.factory.Backup().Restore().UpdateInternal(ctx, id, map[string]interface{}{"name": name})
}

func (c *controller) DeleteRestore(ctx context.Context, id int64) error {
	task, err := c.restore(ctx, id)
	if err != nil {
		return err
	}
	if err = c.ensureClusterAccess(ctx, task.SourceClusterId); err != nil {
		return err
	}
	if err = c.ensureClusterAccess(ctx, task.TargetClusterId); err != nil {
		return err
	}
	client, err := c.velero(ctx, task.TargetClusterId)
	if err != nil {
		return err
	}
	if err = client.DeleteRestore(ctx, task.VeleroNamespace, task.ExternalName); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return c.factory.Backup().Restore().Delete(ctx, id)
}

func (c *controller) ListRestores(ctx context.Context) ([]types.RestoreTask, error) {
	objects, err := c.factory.Backup().Restore().List(ctx, db.WithModifyOrderByDesc())
	if err != nil {
		return nil, err
	}
	result := make([]types.RestoreTask, 0, len(objects))
	for i := range objects {
		if controllerutil.CheckResourceOwner(ctx, objects[i].UserId) == nil && c.ensureClusterAccess(ctx, objects[i].SourceClusterId) == nil && c.ensureClusterAccess(ctx, objects[i].TargetClusterId) == nil {
			c.syncRestore(ctx, &objects[i])
			result = append(result, *toRestore(&objects[i]))
		}
	}
	return result, nil
}

func (c *controller) GetRestore(ctx context.Context, id int64) (*types.RestoreTask, error) {
	task, err := c.restore(ctx, id)
	if err != nil {
		return nil, err
	}
	if err = c.ensureClusterAccess(ctx, task.SourceClusterId); err != nil {
		return nil, err
	}
	if err = c.ensureClusterAccess(ctx, task.TargetClusterId); err != nil {
		return nil, err
	}
	return toRestore(task), nil
}

func (c *controller) velero(ctx context.Context, clusterID int64) (*Client, error) {
	cluster, err := c.factory.Cluster().Get(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	if cluster == nil {
		return nil, fmt.Errorf("cluster %d not found", clusterID)
	}
	clusterSet, err := c.clusters.GetClusterSetByName(ctx, cluster.Name)
	if err != nil {
		return nil, err
	}
	return NewVeleroClient(clusterSet.Config)
}

func (c *controller) ensureClusterAccess(ctx context.Context, clusterID int64) error {
	cluster, err := c.factory.Cluster().Get(ctx, clusterID)
	if err != nil {
		return err
	}
	if cluster == nil {
		return fmt.Errorf("cluster %d not found", clusterID)
	}
	if err = controllerutil.CheckResourceAccess(ctx, c.factory, cluster.UserId, types.ResourceTypeCluster, cluster.Id); err != nil {
		return err
	}
	return controllerutil.CheckMasterKubeconfigAccess(ctx, cluster)
}

func (c *controller) plan(ctx context.Context, id int64) (*model.BackupPlan, error) {
	object, err := c.factory.Backup().Plan().Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if object == nil {
		return nil, fmt.Errorf("backup plan %d not found", id)
	}
	if err = controllerutil.CheckResourceOwner(ctx, object.UserId); err != nil {
		return nil, err
	}
	return object, nil
}
func (c *controller) backup(ctx context.Context, id int64) (*model.Backup, error) {
	object, err := c.factory.Backup().Record().Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if object == nil {
		return nil, fmt.Errorf("backup %d not found", id)
	}
	if err = controllerutil.CheckResourceOwner(ctx, object.UserId); err != nil {
		return nil, err
	}
	return object, nil
}
func (c *controller) restore(ctx context.Context, id int64) (*model.RestoreTask, error) {
	object, err := c.factory.Backup().Restore().Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if object == nil {
		return nil, fmt.Errorf("restore %d not found", id)
	}
	if err = controllerutil.CheckResourceOwner(ctx, object.UserId); err != nil {
		return nil, err
	}
	return object, nil
}

func (c *controller) planSpec(plan *model.BackupPlan, name ...string) BackupSpec {
	var namespaces []string
	_ = json.Unmarshal([]byte(plan.Namespaces), &namespaces)
	var includedResources []string
	_ = json.Unmarshal([]byte(plan.IncludedResources), &includedResources)
	backupName := ""
	if len(name) > 0 {
		backupName = name[0]
	}
	return BackupSpec{Name: backupName, StorageLocation: plan.StorageLocation, Namespaces: namespaces, IncludedResources: includedResources, LabelSelector: plan.LabelSelector, IncludeClusterResources: plan.IncludeClusterResources, SnapshotVolumes: plan.SnapshotVolumes, TTL: fmt.Sprintf("%dh", plan.RetentionDays*24)}
}
func planExternalName(id int64) string { return fmt.Sprintf("pixiu-plan-%d", id) }

func resolveVeleroNamespace(ctx context.Context, client *Client, requested string) (string, error) {
	namespaces, err := client.AvailableNamespaces(ctx)
	if err != nil {
		return "", err
	}
	if namespace := strings.TrimSpace(requested); namespace != "" {
		for _, availableNamespace := range namespaces {
			if namespace == availableNamespace {
				return namespace, nil
			}
		}
		return "", fmt.Errorf("Velero is not ready in namespace %q", namespace)
	}
	switch len(namespaces) {
	case 0:
		return "", fmt.Errorf("Velero is not installed or ready in the cluster")
	case 1:
		return namespaces[0], nil
	default:
		return "", fmt.Errorf("multiple ready Velero installations found; specify a namespace")
	}
}
func normalizeExistingResourceMode(mode string) string {
	if strings.EqualFold(mode, "update") {
		return "update"
	}
	return "none"
}

func (c *controller) syncBackup(ctx context.Context, object *model.Backup) {
	needsErrorDetail := object.Status == model.BackupStatusFailed && strings.TrimSpace(object.ErrorMessage) == ""
	if object.Status != model.BackupStatusNew && object.Status != model.BackupStatusInProgress && !needsErrorDetail {
		return
	}
	client, err := c.velero(ctx, object.SourceClusterId)
	if err != nil {
		return
	}
	resource, err := client.GetBackup(ctx, object.VeleroNamespace, object.ExternalName)
	if err != nil {
		return
	}
	status, warnings, message := backupStatus(resource)
	updates := map[string]interface{}{"status": status, "warnings": warnings, "error_message": message}
	if (status == model.BackupStatusCompleted || status == model.BackupStatusFailed) && object.CompletedAt == nil {
		now := time.Now()
		updates["completed_at"] = now
		object.CompletedAt = &now
	}
	if c.factory.Backup().Record().UpdateInternal(ctx, object.Id, updates) == nil {
		object.Status, object.Warnings, object.ErrorMessage = status, warnings, message
	}
}
func (c *controller) syncRestore(ctx context.Context, object *model.RestoreTask) {
	needsErrorDetail := object.Status == model.RestoreStatusFailed && strings.TrimSpace(object.ErrorMessage) == ""
	if object.Status != model.RestoreStatusNew && object.Status != model.RestoreStatusInProgress && !needsErrorDetail {
		return
	}
	client, err := c.velero(ctx, object.TargetClusterId)
	if err != nil {
		return
	}
	resource, err := client.GetRestore(ctx, object.VeleroNamespace, object.ExternalName)
	if err != nil {
		return
	}
	status, warnings, message := restoreStatus(resource)
	updates := map[string]interface{}{"status": status, "warnings": warnings, "error_message": message}
	if (status == model.RestoreStatusCompleted || status == model.RestoreStatusFailed) && object.CompletedAt == nil {
		now := time.Now()
		updates["completed_at"] = now
		object.CompletedAt = &now
	}
	if c.factory.Backup().Restore().UpdateInternal(ctx, object.Id, updates) == nil {
		object.Status, object.Warnings, object.ErrorMessage = status, warnings, message
	}
}
// resourceErrorDetail 汇总 Velero 资源状态里的失败信息：
// failureReason（Velero 1.10+）、validationErrors（如 BSL 不可用）、errors 字符串或整型计数。
func resourceErrorDetail(resource *unstructured.Unstructured) string {
	parts := make([]string, 0, 4)
	if reason, _, _ := unstructured.NestedString(resource.Object, "status", "failureReason"); strings.TrimSpace(reason) != "" {
		parts = append(parts, strings.TrimSpace(reason))
	}
	if list, _, _ := unstructured.NestedStringSlice(resource.Object, "status", "validationErrors"); len(list) > 0 {
		for _, item := range list {
			if value := strings.TrimSpace(item); value != "" {
				parts = append(parts, value)
			}
		}
	}
	if message, _, _ := unstructured.NestedString(resource.Object, "status", "errors"); strings.TrimSpace(message) != "" {
		parts = append(parts, strings.TrimSpace(message))
	}
	if count, ok, _ := unstructured.NestedInt64(resource.Object, "status", "errors"); ok && count > 0 {
		parts = append(parts, fmt.Sprintf("共 %d 个错误，详见备份日志", count))
	}
	return strings.Join(parts, "\n")
}

func backupStatus(resource *unstructured.Unstructured) (model.BackupStatus, string, string) {
	phase, _, _ := unstructured.NestedString(resource.Object, "status", "phase")
	warnings, _, _ := unstructured.NestedString(resource.Object, "status", "warnings")
	message := resourceErrorDetail(resource)
	switch phase {
	case "Completed":
		return model.BackupStatusCompleted, warnings, message
	case "Failed", "PartiallyFailed", "FailedValidation":
		return model.BackupStatusFailed, warnings, message
	case "InProgress", "WaitingForPluginOperations", "WaitingForPluginOperationsPartiallyFailed":
		return model.BackupStatusInProgress, warnings, message
	default:
		return model.BackupStatusInProgress, warnings, message
	}
}
func restoreStatus(resource *unstructured.Unstructured) (model.RestoreStatus, string, string) {
	phase, _, _ := unstructured.NestedString(resource.Object, "status", "phase")
	warnings, _, _ := unstructured.NestedString(resource.Object, "status", "warnings")
	message := resourceErrorDetail(resource)
	switch phase {
	case "Completed":
		return model.RestoreStatusCompleted, warnings, message
	case "Failed", "PartiallyFailed", "FailedValidation":
		return model.RestoreStatusFailed, warnings, message
	case "InProgress", "WaitingForPluginOperations", "WaitingForPluginOperationsPartiallyFailed":
		return model.RestoreStatusInProgress, warnings, message
	default:
		return model.RestoreStatusInProgress, warnings, message
	}
}

func applyPlanUpdates(plan *model.BackupPlan, updates map[string]interface{}) {
	if value, ok := updates["source_cluster_id"].(int64); ok {
		plan.SourceClusterId = value
	}
	if value, ok := updates["storage_datasource_id"].(int64); ok {
		plan.StorageDatasourceId = value
	}
	if value, ok := updates["storage_location"].(string); ok {
		plan.StorageLocation = value
	}
	if value, ok := updates["storage_bucket"].(string); ok {
		plan.StorageBucket = value
	}
	if value, ok := updates["storage_config"].(string); ok {
		plan.StorageConfig = value
	}
	if value, ok := updates["velero_namespace"].(string); ok {
		plan.VeleroNamespace = value
	}
	if value, ok := updates["schedule"].(string); ok {
		plan.Schedule = value
	}
	if value, ok := updates["retention_days"].(int); ok {
		plan.RetentionDays = value
	}
	if value, ok := updates["namespaces"].(string); ok {
		plan.Namespaces = value
	}
	if value, ok := updates["included_resources"].(string); ok {
		plan.IncludedResources = value
	}
	if value, ok := updates["label_selector"].(string); ok {
		plan.LabelSelector = value
	}
	if value, ok := updates["include_cluster_resources"].(bool); ok {
		plan.IncludeClusterResources = value
	}
	if value, ok := updates["snapshot_volumes"].(bool); ok {
		plan.SnapshotVolumes = value
	}
	if value, ok := updates["enabled"].(bool); ok {
		plan.Enabled = value
	}
}

func toPlan(object *model.BackupPlan) *types.BackupPlan {
	var namespaces []string
	_ = json.Unmarshal([]byte(object.Namespaces), &namespaces)
	var includedResources []string
	_ = json.Unmarshal([]byte(object.IncludedResources), &includedResources)
	return &types.BackupPlan{PixiuMeta: types.PixiuMeta{Id: object.Id, ResourceVersion: object.ResourceVersion}, TimeMeta: types.TimeMeta{GmtCreate: object.GmtCreate, GmtModified: object.GmtModified}, Name: object.Name, Description: object.Description, UserId: object.UserId, SourceClusterId: object.SourceClusterId, StorageDatasourceId: object.StorageDatasourceId, StorageBucket: object.StorageBucket, StorageLocation: object.StorageLocation, StorageConfig: storageConfigFromTypesJSON(object.StorageConfig), VeleroNamespace: object.VeleroNamespace, Schedule: object.Schedule, RetentionDays: object.RetentionDays, Namespaces: namespaces, IncludedResources: includedResources, LabelSelector: object.LabelSelector, IncludeClusterResources: object.IncludeClusterResources, SnapshotVolumes: object.SnapshotVolumes, Enabled: object.Enabled, RestoreTargetClusterId: object.RestoreTargetClusterId, RestoreNamespace: object.RestoreNamespace, RestoreExistingResourceMode: object.RestoreExistingResourceMode, RestoreVolumes: object.RestoreVolumes}
}

func (c *controller) storageConfigFromDatasource(ctx context.Context, datasourceID int64, bucket string) (StorageConfig, string, string, error) {
	storage, err := c.storage.ResolveConfig(ctx, datasourceID)
	if err != nil {
		return StorageConfig{}, "", "", err
	}
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		bucket = strings.TrimSpace(storage.Bucket)
	}
	if strings.TrimSpace(storage.Endpoint) == "" || bucket == "" || strings.TrimSpace(storage.AccessKeyID) == "" || strings.TrimSpace(storage.SecretAccessKey) == "" {
		return StorageConfig{}, "", "", fmt.Errorf("storage datasource %d must include endpoint, bucket and credentials", datasourceID)
	}
	provider := strings.ToLower(strings.TrimSpace(storage.Provider))
	if provider == "aws" || provider == "minio" {
		provider = "s3"
	}
	config := StorageConfig{Provider: provider, Endpoint: storage.Endpoint, Bucket: bucket, Region: storage.Region, AccessKey: storage.AccessKeyID, SecretKey: storage.SecretAccessKey, SessionToken: storage.SessionToken, UsePathStyle: storage.AddressingStyle == "path" || strings.EqualFold(storage.Provider, "minio")}
	return config, bucket, storageLocationName(datasourceID, bucket), nil
}

func (c *controller) planStorageConfig(ctx context.Context, plan *model.BackupPlan) (StorageConfig, error) {
	if plan.StorageDatasourceId == 0 {
		return storageConfigFromJSON(plan.StorageConfig), nil
	}
	config, _, _, err := c.storageConfigFromDatasource(ctx, plan.StorageDatasourceId, plan.StorageBucket)
	return config, err
}

func (c *controller) backupStorageConfig(ctx context.Context, backup *model.Backup) (StorageConfig, error) {
	if backup.StorageDatasourceId == 0 {
		return storageConfigFromJSON(backup.StorageConfig), nil
	}
	config, _, _, err := c.storageConfigFromDatasource(ctx, backup.StorageDatasourceId, backup.StorageBucket)
	return config, err
}

func storageLocationName(datasourceID int64, bucket string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(bucket)))
	return fmt.Sprintf("pixiu-storage-%d-%x", datasourceID, digest[:6])
}

func storageConfigToEngine(config types.BackupStorageConfig) StorageConfig {
	return StorageConfig{Provider: config.Provider, Endpoint: config.Endpoint, Bucket: config.Bucket, Prefix: config.Prefix, Region: config.Region, AccessKey: config.AccessKey, SecretKey: config.SecretKey, SessionToken: config.SessionToken, UsePathStyle: config.UsePathStyle, InsecureSkipTLSVerify: config.InsecureSkipTLSVerify}
}

func storageConfigFromJSON(raw string) StorageConfig {
	var config types.BackupStorageConfig
	_ = json.Unmarshal([]byte(raw), &config)
	return storageConfigToEngine(config)
}
func storageConfigFromTypesJSON(raw string) types.BackupStorageConfig {
	var config types.BackupStorageConfig
	_ = json.Unmarshal([]byte(raw), &config)
	config.SecretKey = ""
	return config
}
func restoreVolumesValue(value *bool) bool {
	if value == nil {
		return true
	}
	return *value
}
func toBackup(object *model.Backup) *types.Backup {
	return &types.Backup{PixiuMeta: types.PixiuMeta{Id: object.Id, ResourceVersion: object.ResourceVersion}, TimeMeta: types.TimeMeta{GmtCreate: object.GmtCreate, GmtModified: object.GmtModified}, PlanId: object.PlanId, SourceClusterId: object.SourceClusterId, StorageDatasourceId: object.StorageDatasourceId, StorageBucket: object.StorageBucket, StorageLocation: object.StorageLocation, VeleroNamespace: object.VeleroNamespace, ExternalName: object.ExternalName, Status: object.Status, StartedAt: object.StartedAt, CompletedAt: object.CompletedAt, ExpiresAt: object.ExpiresAt, Warnings: object.Warnings, ErrorMessage: object.ErrorMessage}
}
func toRestore(object *model.RestoreTask) *types.RestoreTask {
	var namespaceMapping, storageClassMapping map[string]string
	var includedNamespaces, excludedNamespaces []string
	_ = json.Unmarshal([]byte(object.NamespaceMapping), &namespaceMapping)
	_ = json.Unmarshal([]byte(object.StorageClassMapping), &storageClassMapping)
	_ = json.Unmarshal([]byte(object.IncludedNamespaces), &includedNamespaces)
	_ = json.Unmarshal([]byte(object.ExcludedNamespaces), &excludedNamespaces)
	return &types.RestoreTask{PixiuMeta: types.PixiuMeta{Id: object.Id, ResourceVersion: object.ResourceVersion}, TimeMeta: types.TimeMeta{GmtCreate: object.GmtCreate, GmtModified: object.GmtModified}, Name: object.Name, Description: object.Description, BackupId: object.BackupId, SourceClusterId: object.SourceClusterId, TargetClusterId: object.TargetClusterId, VeleroNamespace: object.VeleroNamespace, ExternalName: object.ExternalName, IncludedNamespaces: includedNamespaces, ExcludedNamespaces: excludedNamespaces, NamespaceMapping: namespaceMapping, StorageClassMapping: storageClassMapping, ExistingResourceMode: object.ExistingResourceMode, RestoreVolumes: object.RestoreVolumes, Status: object.Status, StartedAt: object.StartedAt, CompletedAt: object.CompletedAt, Warnings: object.Warnings, ErrorMessage: object.ErrorMessage}
}
