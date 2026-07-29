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

type DeployAgentInterface interface {
	Create(ctx context.Context, object *model.DeployAgent) (*model.DeployAgent, error)
	Update(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error
	InternalUpdate(ctx context.Context, id int64, updates map[string]interface{}) error
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (*model.DeployAgent, error)
	GetByToken(ctx context.Context, token string) (*model.DeployAgent, error)
	List(ctx context.Context, opts ...Options) ([]model.DeployAgent, error)
}

type DeployJobInterface interface {
	Create(ctx context.Context, object *model.DeployJob) (*model.DeployJob, error)
	Get(ctx context.Context, id int64) (*model.DeployJob, error)
	InternalUpdate(ctx context.Context, id int64, updates map[string]interface{}) error
	ClaimNext(ctx context.Context, agentId int64) (*model.DeployJob, error)
	AppendLogs(ctx context.Context, id int64, chunk string) error
	ListByPlan(ctx context.Context, planId int64) ([]model.DeployJob, error)
}

type deployAgent struct{ db *gorm.DB }
type deployJob struct{ db *gorm.DB }

func newDeployAgent(db *gorm.DB) DeployAgentInterface { return &deployAgent{db: db} }
func newDeployJob(db *gorm.DB) DeployJobInterface     { return &deployJob{db: db} }

func (a *deployAgent) Create(ctx context.Context, object *model.DeployAgent) (*model.DeployAgent, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now
	if err := a.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (a *deployAgent) Update(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error {
	updates["resource_version"] = resourceVersion + 1
	updates["gmt_modified"] = time.Now()
	f := a.db.WithContext(ctx).Model(&model.DeployAgent{}).
		Where("id = ? AND resource_version = ?", id, resourceVersion).Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotUpdate
	}
	return nil
}

func (a *deployAgent) InternalUpdate(ctx context.Context, id int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	return a.db.WithContext(ctx).Model(&model.DeployAgent{}).Where("id = ?", id).Updates(updates).Error
}

func (a *deployAgent) Delete(ctx context.Context, id int64) error {
	return a.db.WithContext(ctx).Where("id = ?", id).Delete(&model.DeployAgent{}).Error
}

func (a *deployAgent) Get(ctx context.Context, id int64) (*model.DeployAgent, error) {
	var object model.DeployAgent
	if err := a.db.WithContext(ctx).Where("id = ?", id).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (a *deployAgent) GetByToken(ctx context.Context, token string) (*model.DeployAgent, error) {
	if token == "" {
		return nil, nil
	}
	var object model.DeployAgent
	if err := a.db.WithContext(ctx).Where("token = ?", token).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (a *deployAgent) List(ctx context.Context, opts ...Options) ([]model.DeployAgent, error) {
	var objects []model.DeployAgent
	tx := a.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Find(&objects).Error; err != nil {
		return nil, err
	}
	return objects, nil
}

func (j *deployJob) Create(ctx context.Context, object *model.DeployJob) (*model.DeployJob, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now
	if object.Status == "" {
		object.Status = model.DeployJobPending
	}
	if err := j.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (j *deployJob) Get(ctx context.Context, id int64) (*model.DeployJob, error) {
	var object model.DeployJob
	if err := j.db.WithContext(ctx).Where("id = ?", id).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (j *deployJob) InternalUpdate(ctx context.Context, id int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	return j.db.WithContext(ctx).Model(&model.DeployJob{}).Where("id = ?", id).Updates(updates).Error
}

func (j *deployJob) ClaimNext(ctx context.Context, agentId int64) (*model.DeployJob, error) {
	var object model.DeployJob
	err := j.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("agent_id = ? AND status = ?", agentId, model.DeployJobPending).
			Order("id ASC").First(&object).Error; err != nil {
			return err
		}
		now := time.Now()
		return tx.Model(&model.DeployJob{}).Where("id = ? AND status = ?", object.Id, model.DeployJobPending).
			Updates(map[string]interface{}{
				"status":       model.DeployJobRunning,
				"claimed_at":   now,
				"gmt_modified": now,
			}).Error
	})
	if err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	object.Status = model.DeployJobRunning
	return &object, nil
}

func (j *deployJob) AppendLogs(ctx context.Context, id int64, chunk string) error {
	if chunk == "" {
		return nil
	}
	return j.db.WithContext(ctx).Exec(
		"UPDATE deploy_jobs SET logs = CONCAT(IFNULL(logs,''), ?), gmt_modified = ? WHERE id = ?",
		chunk, time.Now(), id,
	).Error
}

func (j *deployJob) ListByPlan(ctx context.Context, planId int64) ([]model.DeployJob, error) {
	var objects []model.DeployJob
	if err := j.db.WithContext(ctx).Where("plan_id = ?", planId).Order("id ASC").Find(&objects).Error; err != nil {
		return nil, err
	}
	return objects, nil
}
