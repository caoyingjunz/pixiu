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

package upgrade

import (
	"context"
	"fmt"
	"net/http"
	"time"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

// Interface 系统升级状态业务接口。
// 所有状态迁移均使用条件 UPDATE + RowsAffected 判定，禁止整行覆盖写入。
type Interface interface {
	// Create 为一次新的版本探测创建记录
	Create(ctx context.Context, object *model.UpgradeRecord) error
	// GetBy 按条件返回最新的一条升级记录；未传条件时取 id 最大的一条；不存在时返回 nil, nil。
	GetBy(ctx context.Context, opts ...db.Options) (*model.UpgradeRecord, error)
	// Latest 返回最新一条升级状态；存在升级流程中的记录时优先返回该记录。
	Latest(ctx context.Context) (*types.UpgradeStatus, error)
	// TriggerUpgrade 校验并原子迁移到待升级（0/4 -> 1）。
	TriggerUpgrade(ctx context.Context) error
	// Reset 升级器启动时清理自身上次崩溃遗留
	Reset(ctx context.Context, upgradeID, message string) (bool, error)
	// Claim 升级进程原子认领；未抢到返回 nil, nil
	Claim(ctx context.Context, upgradeID string) (*model.UpgradeRecord, error)

	// MarkPending 升级状态调整
	MarkPending(ctx context.Context) (bool, error)
	MarkCompleted(ctx context.Context, upgradeID, version string) (bool, error)
	MarkFailed(ctx context.Context, upgradeID, message string) (bool, error)

	// CurrentVersion 返回当前使用版本的版本号；无记录时返回空字符串。
	CurrentVersion(ctx context.Context) (string, error)
}

type Getter interface {
	Upgrade() Interface
}

type upgrade struct {
	dao db.UpgradeInterface
}

func New(dao db.UpgradeInterface) Interface {
	return &upgrade{dao: dao}
}

// GetBy 按条件返回最新的一条升级记录；未传条件时取 id 最大的一条；不存在时返回 nil, nil。
func (u *upgrade) GetBy(ctx context.Context, opts ...db.Options) (*model.UpgradeRecord, error) {
	q := make([]db.Options, 0, len(opts)+1)
	q = append(q, db.WithOrderByDesc())
	q = append(q, opts...)
	return u.dao.GetBy(ctx, q...)
}

func (u *upgrade) Latest(ctx context.Context) (*types.UpgradeStatus, error) {
	record, err := u.GetBy(ctx, db.WithUpgradeStatus(model.UpgradeStatusPending, model.UpgradeStatusUpgrading))
	if err != nil {
		return nil, err
	}
	if record == nil {
		if record, err = u.GetBy(ctx); err != nil {
			return nil, err
		}
	}
	return upgradeStatus(record), nil
}

func upgradeStatus(record *model.UpgradeRecord) *types.UpgradeStatus {
	if record == nil {
		return &types.UpgradeStatus{}
	}
	return &types.UpgradeStatus{
		IsCurrent:    record.IsCurrent,
		VersionTag:   record.VersionTag,
		CommitID:     record.CommitID,
		Image:        record.Image,
		Status:       record.Status,
		Upgrading:    record.Status == model.UpgradeStatusPending || record.Status == model.UpgradeStatusUpgrading,
		ErrorMessage: record.ErrorMessage,
		ReleaseTime:  record.ReleaseTime,
	}
}

func (u *upgrade) TriggerUpgrade(ctx context.Context) error {
	active, err := u.GetBy(ctx, db.WithUpgradeStatus(model.UpgradeStatusPending, model.UpgradeStatusUpgrading))
	if err != nil {
		return err
	}
	if active != nil {
		return apierrors.NewError(fmt.Errorf("升级已在进行中"), http.StatusConflict)
	}

	record, err := u.GetBy(ctx)
	if err != nil {
		return err
	}
	switch {
	case record == nil || record.Image == "" || record.VersionTag == "" || record.IsCurrent:
		return apierrors.NewError(fmt.Errorf("当前已是最新版本"), http.StatusConflict)
	case record.Status != model.UpgradeStatusNotUpgraded && record.Status != model.UpgradeStatusFailed:
		return apierrors.NewError(fmt.Errorf("当前已是最新版本"), http.StatusConflict)
	}

	marked, err := u.MarkPending(ctx)
	if err != nil {
		return err
	}
	if !marked {
		return apierrors.NewError(fmt.Errorf("升级状态已变化，请刷新后重试"), http.StatusConflict)
	}
	return nil
}

// update 对最新一条匹配记录执行条件更新，返回受影响行数；无匹配时返回 0, nil。
func (u *upgrade) update(ctx context.Context, updates map[string]interface{}, opts ...db.Options) (int64, error) {
	record, err := u.GetBy(ctx, opts...)
	if err != nil || record == nil {
		return 0, err
	}
	return u.dao.Update(ctx, record.Id, updates, opts...)
}

// Create 为一次新的版本探测创建记录（每个 version+commit 一行）。
func (u *upgrade) Create(ctx context.Context, object *model.UpgradeRecord) error {
	if object == nil {
		return fmt.Errorf("保存版本探测结果失败: 记录为空")
	}
	return u.dao.Create(ctx, object)
}

func (u *upgrade) MarkPending(ctx context.Context) (bool, error) {
	record, err := u.GetBy(ctx)
	if err != nil {
		return false, err
	}
	if record == nil ||
		(record.Status != model.UpgradeStatusNotUpgraded && record.Status != model.UpgradeStatusFailed) ||
		record.Image == "" || record.VersionTag == "" || record.IsCurrent {
		return false, nil
	}

	affected, err := u.dao.Update(ctx, record.Id, map[string]interface{}{
		"status":        model.UpgradeStatusPending,
		"error_message": "",
		"upgrade_id":    "",
		"gmt_modified":  time.Now(),
	},
		db.WithUpgradeStatus(model.UpgradeStatusNotUpgraded, model.UpgradeStatusFailed),
		db.WithUpgradeTargetReady(),
	)
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}

// Claim 原子认领待升级请求：仅当记录处于「待升级」时推进到「升级中」，并记录执行者标识。
// 未抢到返回 nil, nil；返回非空记录即代表本次认领成功。
func (u *upgrade) Claim(ctx context.Context, upgradeID string) (*model.UpgradeRecord, error) {
	affected, err := u.update(ctx, map[string]interface{}{
		"status":        model.UpgradeStatusUpgrading,
		"upgrade_id":    upgradeID,
		"error_message": "",
		"gmt_modified":  time.Now(),
	}, db.WithUpgradeStatus(model.UpgradeStatusPending))
	if err != nil {
		return nil, err
	}
	if affected != 1 {
		return nil, nil
	}

	// 按本次认领标识精确回查，避免依赖「单行/首行」假设拿到其他记录
	records, err := u.dao.List(ctx, db.WithUpgradeID(upgradeID), db.WithUpgradeStatus(model.UpgradeStatusUpgrading), db.WithLimit(1))
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("升级请求已认领但未读取到记录")
	}
	return &records[0], nil
}

func (u *upgrade) MarkCompleted(ctx context.Context, upgradeID, version string) (bool, error) {
	record, err := u.GetBy(ctx,
		db.WithUpgradeStatus(model.UpgradeStatusUpgrading),
		db.WithUpgradeID(upgradeID),
		db.WithUpgradeVersionTag(version),
	)
	if err != nil {
		return false, err
	}
	if record == nil {
		return false, nil
	}

	affected, err := u.dao.MarkCompleted(ctx, record.Id, upgradeID, version)
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}

func (u *upgrade) CurrentVersion(ctx context.Context) (string, error) {
	record, err := u.GetBy(ctx, db.WithUpgradeIsCurrent(true))
	if err != nil {
		return "", err
	}
	if record == nil {
		return "", nil
	}
	return record.VersionTag, nil
}

func (u *upgrade) MarkFailed(ctx context.Context, upgradeID, message string) (bool, error) {
	affected, err := u.update(ctx, map[string]interface{}{
		"status":        model.UpgradeStatusFailed,
		"error_message": message,
		"gmt_modified":  time.Now(),
	},
		db.WithUpgradeStatus(model.UpgradeStatusUpgrading, model.UpgradeStatusCompleted),
		db.WithUpgradeID(upgradeID),
	)
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}

func (u *upgrade) Reset(ctx context.Context, upgradeID, message string) (bool, error) {
	affected, err := u.update(ctx, map[string]interface{}{
		"status":        model.UpgradeStatusFailed,
		"error_message": message,
		"gmt_modified":  time.Now(),
	},
		db.WithUpgradeStatus(model.UpgradeStatusUpgrading),
		db.WithUpgradeID(upgradeID),
	)
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}
