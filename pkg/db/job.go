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

type JobInterface interface {
	Create(ctx context.Context, object *model.Job) (*model.Job, error)
	Get(ctx context.Context, id int64) (*model.Job, error)
	ClaimNext(ctx context.Context, agentId int64) (*model.Job, error)
	AppendLogs(ctx context.Context, id int64, chunk string) error
	ListByPlan(ctx context.Context, planId int64) ([]model.Job, error)

	InternalUpdate(ctx context.Context, id int64, updates map[string]interface{}) error
}

type job struct{ db *gorm.DB }

func newJob(db *gorm.DB) JobInterface { return &job{db: db} }

func (j *job) Create(ctx context.Context, object *model.Job) (*model.Job, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now
	if object.Status == "" {
		object.Status = model.JobPending
	}
	if err := j.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (j *job) Get(ctx context.Context, id int64) (*model.Job, error) {
	var object model.Job
	if err := j.db.WithContext(ctx).Where("id = ?", id).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (j *job) InternalUpdate(ctx context.Context, id int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	return j.db.WithContext(ctx).Model(&model.Job{}).Where("id = ?", id).Updates(updates).Error
}

// ClaimNext 在事务中原子地认领该 Agent 的下一个待处理作业，将其状态从 Pending 切换为 Running。
func (j *job) ClaimNext(ctx context.Context, agentId int64) (*model.Job, error) {
	var object model.Job
	err := j.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("agent_id = ? AND status = ?", agentId, model.JobPending).
			Order("id ASC").First(&object).Error; err != nil {
			return err
		}
		now := time.Now()
		return tx.Model(&model.Job{}).Where("id = ? AND status = ?", object.Id, model.JobPending).
			Updates(map[string]interface{}{
				"status":       model.JobRunning,
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
	object.Status = model.JobRunning
	return &object, nil
}

func (j *job) AppendLogs(ctx context.Context, id int64, chunk string) error {
	if chunk == "" {
		return nil
	}
	return j.db.WithContext(ctx).Exec(
		"UPDATE deploy_jobs SET logs = CONCAT(IFNULL(logs,''), ?), gmt_modified = ? WHERE id = ?",
		chunk, time.Now(), id,
	).Error
}

func (j *job) ListByPlan(ctx context.Context, planId int64) ([]model.Job, error) {
	var objects []model.Job
	if err := j.db.WithContext(ctx).Where("plan_id = ?", planId).Order("id ASC").Find(&objects).Error; err != nil {
		return nil, err
	}
	return objects, nil
}
