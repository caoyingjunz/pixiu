/*
Copyright 2026 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package db

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/util/errors"
)

// CICDInterface 流水线 / 运行 / 触发器三张表的数据访问。
type CICDInterface interface {
	// Pipeline
	CreatePipeline(ctx context.Context, object *model.CICDPipeline) (*model.CICDPipeline, error)
	GetPipeline(ctx context.Context, id int64) (*model.CICDPipeline, error)
	GetPipelineByName(ctx context.Context, tenantId int64, name string) (*model.CICDPipeline, error)
	ListPipelines(ctx context.Context, opts ...Options) ([]model.CICDPipeline, error)
	CountPipelines(ctx context.Context, opts ...Options) (int64, error)
	UpdatePipeline(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) (bool, error)
	DeletePipeline(ctx context.Context, id int64) error

	// Run
	CreateRun(ctx context.Context, object *model.CICDRun) (*model.CICDRun, error)
	GetRun(ctx context.Context, id int64) (*model.CICDRun, error)
	GetRunByEngineName(ctx context.Context, engineName string) (*model.CICDRun, error)
	ListRuns(ctx context.Context, opts ...Options) ([]model.CICDRun, error)
	CountRuns(ctx context.Context, opts ...Options) (int64, error)
	// ClaimRun 原子领取 pending run：仅当状态仍为 pending 时置为 running，返回是否领取成功。
	ClaimRun(ctx context.Context, id int64, updates map[string]interface{}) (bool, error)
	UpdateRun(ctx context.Context, id int64, updates map[string]interface{}) error
	ListPendingRuns(ctx context.Context, limit int) ([]model.CICDRun, error)
	ListRunningRuns(ctx context.Context, limit int) ([]model.CICDRun, error)
	// ListNonTerminalRunPipelineIDs 返回存在非终态 run 的 pipeline id 集合，用于单流水线并发限制。
	ListNonTerminalRunPipelineIDs(ctx context.Context) ([]int64, error)
	// MarkStaleRunningFailed 启动对账前把长时间卡在 running 且旧于 cutoff 的记录置为 failed。
	MarkStaleRunningFailed(ctx context.Context, cutoff time.Time) (int64, error)
	CountRunningByPipeline(ctx context.Context, pipelineId int64) (int64, error)
	CountPending(ctx context.Context) (int64, error)

	// Trigger
	CreateTrigger(ctx context.Context, object *model.CICDTrigger) (*model.CICDTrigger, error)
	GetTrigger(ctx context.Context, id int64) (*model.CICDTrigger, error)
	ListTriggers(ctx context.Context, opts ...Options) ([]model.CICDTrigger, error)
	UpdateTrigger(ctx context.Context, id int64, updates map[string]interface{}) error
	DeleteTrigger(ctx context.Context, id int64) error
}

type cicd struct{ db *gorm.DB }

func newCICD(db *gorm.DB) CICDInterface { return &cicd{db: db} }

var _ CICDInterface = &cicd{}

// ---- Pipeline ----

func (c *cicd) CreatePipeline(ctx context.Context, object *model.CICDPipeline) (*model.CICDPipeline, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now
	if object.Engine == "" {
		object.Engine = "tekton"
	}
	if object.SpecMode == "" {
		object.SpecMode = "template"
	}
	if object.Template == "" {
		object.Template = "source-build"
	}
	if object.TimeoutSeconds <= 0 {
		object.TimeoutSeconds = 1200
	}
	if err := c.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (c *cicd) GetPipeline(ctx context.Context, id int64) (*model.CICDPipeline, error) {
	var object model.CICDPipeline
	if err := c.db.WithContext(ctx).Where("id = ?", id).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (c *cicd) GetPipelineByName(ctx context.Context, tenantId int64, name string) (*model.CICDPipeline, error) {
	var object model.CICDPipeline
	if err := c.db.WithContext(ctx).Where("tenant_id = ? AND name = ?", tenantId, name).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (c *cicd) ListPipelines(ctx context.Context, opts ...Options) ([]model.CICDPipeline, error) {
	var objects []model.CICDPipeline
	tx := c.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Order("id DESC").Find(&objects).Error; err != nil {
		return nil, err
	}
	return objects, nil
}

func (c *cicd) CountPipelines(ctx context.Context, opts ...Options) (int64, error) {
	var count int64
	tx := c.db.WithContext(ctx).Model(&model.CICDPipeline{})
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// UpdatePipeline 以 resource_version 乐观锁更新，冲突返回 false。
func (c *cicd) UpdatePipeline(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) (bool, error) {
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1
	f := c.db.WithContext(ctx).Model(&model.CICDPipeline{}).
		Where("id = ? AND resource_version = ?", id, resourceVersion).
		Updates(updates)
	if f.Error != nil {
		return false, f.Error
	}
	return f.RowsAffected == 1, nil
}

func (c *cicd) DeletePipeline(ctx context.Context, id int64) error {
	return c.db.WithContext(ctx).Where("id = ?", id).Delete(&model.CICDPipeline{}).Error
}

// ---- Run ----

func (c *cicd) CreateRun(ctx context.Context, object *model.CICDRun) (*model.CICDRun, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now
	if object.Status == "" {
		object.Status = model.CICDRunPending
	}
	if object.Engine == "" {
		object.Engine = "tekton"
	}
	if err := c.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (c *cicd) GetRun(ctx context.Context, id int64) (*model.CICDRun, error) {
	var object model.CICDRun
	if err := c.db.WithContext(ctx).Where("id = ?", id).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (c *cicd) GetRunByEngineName(ctx context.Context, engineName string) (*model.CICDRun, error) {
	var object model.CICDRun
	if err := c.db.WithContext(ctx).Where("engine_name = ?", engineName).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (c *cicd) ListRuns(ctx context.Context, opts ...Options) ([]model.CICDRun, error) {
	var objects []model.CICDRun
	tx := c.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Order("id DESC").Find(&objects).Error; err != nil {
		return nil, err
	}
	return objects, nil
}

func (c *cicd) CountRuns(ctx context.Context, opts ...Options) (int64, error) {
	var count int64
	tx := c.db.WithContext(ctx).Model(&model.CICDRun{})
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (c *cicd) ClaimRun(ctx context.Context, id int64, updates map[string]interface{}) (bool, error) {
	updates["gmt_modified"] = time.Now()
	f := c.db.WithContext(ctx).Model(&model.CICDRun{}).
		Where("id = ? AND status = ?", id, model.CICDRunPending).
		Updates(updates)
	if f.Error != nil {
		return false, f.Error
	}
	return f.RowsAffected == 1, nil
}

func (c *cicd) UpdateRun(ctx context.Context, id int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	return c.db.WithContext(ctx).Model(&model.CICDRun{}).Where("id = ?", id).Updates(updates).Error
}

func (c *cicd) ListPendingRuns(ctx context.Context, limit int) ([]model.CICDRun, error) {
	var objects []model.CICDRun
	tx := c.db.WithContext(ctx).Where("status = ?", model.CICDRunPending).Order("id ASC")
	if limit > 0 {
		tx = tx.Limit(limit)
	}
	if err := tx.Find(&objects).Error; err != nil {
		return nil, err
	}
	return objects, nil
}

func (c *cicd) ListRunningRuns(ctx context.Context, limit int) ([]model.CICDRun, error) {
	var objects []model.CICDRun
	tx := c.db.WithContext(ctx).Where("status = ?", model.CICDRunRunning).Order("id ASC")
	if limit > 0 {
		tx = tx.Limit(limit)
	}
	if err := tx.Find(&objects).Error; err != nil {
		return nil, err
	}
	return objects, nil
}

func (c *cicd) ListNonTerminalRunPipelineIDs(ctx context.Context) ([]int64, error) {
	var ids []int64
	err := c.db.WithContext(ctx).Model(&model.CICDRun{}).
		Where("status IN ?", []model.CICDRunStatus{model.CICDRunPending, model.CICDRunRunning}).
		Distinct("pipeline_id").Pluck("pipeline_id", &ids).Error
	return ids, err
}

func (c *cicd) MarkStaleRunningFailed(ctx context.Context, cutoff time.Time) (int64, error) {
	now := time.Now()
	f := c.db.WithContext(ctx).Model(&model.CICDRun{}).
		Where("status = ? AND gmt_modified < ?", model.CICDRunRunning, cutoff).
		Updates(map[string]interface{}{
			"status":      model.CICDRunFailed,
			"message":     "运行超时或控制面重启，状态已收口为 failed",
			"finished_at": now,
			"gmt_modified": now,
		})
	return f.RowsAffected, f.Error
}

func (c *cicd) CountRunningByPipeline(ctx context.Context, pipelineId int64) (int64, error) {
	var count int64
	err := c.db.WithContext(ctx).Model(&model.CICDRun{}).
		Where("pipeline_id = ? AND status IN ?", pipelineId, []model.CICDRunStatus{model.CICDRunPending, model.CICDRunRunning}).
		Count(&count).Error
	return count, err
}

func (c *cicd) CountPending(ctx context.Context) (int64, error) {
	var count int64
	err := c.db.WithContext(ctx).Model(&model.CICDRun{}).Where("status = ?", model.CICDRunPending).Count(&count).Error
	return count, err
}

// ---- Trigger ----

func (c *cicd) CreateTrigger(ctx context.Context, object *model.CICDTrigger) (*model.CICDTrigger, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now
	if err := c.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (c *cicd) GetTrigger(ctx context.Context, id int64) (*model.CICDTrigger, error) {
	var object model.CICDTrigger
	if err := c.db.WithContext(ctx).Where("id = ?", id).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (c *cicd) ListTriggers(ctx context.Context, opts ...Options) ([]model.CICDTrigger, error) {
	var objects []model.CICDTrigger
	tx := c.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Order("id DESC").Find(&objects).Error; err != nil {
		return nil, err
	}
	return objects, nil
}

func (c *cicd) UpdateTrigger(ctx context.Context, id int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	return c.db.WithContext(ctx).Model(&model.CICDTrigger{}).Where("id = ?", id).Updates(updates).Error
}

func (c *cicd) DeleteTrigger(ctx context.Context, id int64) error {
	return c.db.WithContext(ctx).Where("id = ?", id).Delete(&model.CICDTrigger{}).Error
}

// ---- CICD 专属过滤 Options ----

func WithCICDPipelineId(id int64) Options {
	return func(tx *gorm.DB) *gorm.DB {
		return tx.Where("pipeline_id = ?", id)
	}
}

func WithCICDRunStatus(status model.CICDRunStatus) Options {
	return func(tx *gorm.DB) *gorm.DB {
		if status == "" {
			return tx
		}
		return tx.Where("status = ?", status)
	}
}

func WithCICDTriggerPipelineId(id int64) Options {
	return func(tx *gorm.DB) *gorm.DB {
		return tx.Where("pipeline_id = ?", id)
	}
}

func WithCICDPipelineNamespace(namespace string) Options {
	return func(tx *gorm.DB) *gorm.DB {
		if namespace == "" {
			return tx
		}
		return tx.Where("namespace = ?", namespace)
	}
}

func WithCICDCreatedBy(userId int64) Options {
	return func(tx *gorm.DB) *gorm.DB {
		return tx.Where("created_by = ?", userId)
	}
}
