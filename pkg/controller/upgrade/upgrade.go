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
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

// upgradeRecordID 单行升级记录的业务约定：升级状态固定存放在 id=1 的记录中。
const upgradeRecordID int64 = 1

// upgradeFlowStatuses 升级流程进行中的状态，检查器不得覆盖。
var upgradeFlowStatuses = []int{model.UpgradeStatusPending, model.UpgradeStatusUpgrading}

// Interface 系统升级状态业务接口。
// 所有状态迁移均使用条件 UPDATE + RowsAffected 判定，禁止整行覆盖写入。
type Interface interface {
	// Get 返回升级状态；记录不存在时返回 nil, nil。
	Get(ctx context.Context) (*model.UpgradeRecord, error)
	// SaveCheck 保存版本检查结果；升级流程进行中（状态 1/2）时只刷新 last_checked_at。
	SaveCheck(ctx context.Context, object *model.UpgradeRecord) error
	// SaveCheckError 记录版本检查失败信息；升级流程进行中时忽略。
	SaveCheckError(ctx context.Context, message string) error
	// MarkPending 用户确认升级：0/4 -> 1。
	MarkPending(ctx context.Context) (bool, error)
	// Claim upgrade 进程原子认领升级请求：1 -> 2，并写入 upgrade_id；未抢到返回 nil, false, nil。
	Claim(ctx context.Context, upgradeID string) (*model.UpgradeRecord, bool, error)
	// MarkCompleted 新版 pixiu 回写升级完成：2 -> 3。
	MarkCompleted(ctx context.Context, upgradeID, version string) (bool, error)
	// MarkFailed 升级器回写升级失败：2/3 -> 4（允许 3 -> 4：pixiu 先回写 3 后健康检查失败）。
	MarkFailed(ctx context.Context, upgradeID, message string) (bool, error)
	// ResetStale 升级器启动时清理自身上次崩溃遗留的 2：2 -> 4。
	ResetStale(ctx context.Context, upgradeID, message string) (bool, error)
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

func (u *upgrade) Get(ctx context.Context) (*model.UpgradeRecord, error) {
	return u.dao.Get(ctx, upgradeRecordID)
}

func (u *upgrade) SaveCheck(ctx context.Context, object *model.UpgradeRecord) error {
	existing, err := u.dao.Get(ctx, upgradeRecordID)
	if err != nil {
		return err
	}

	now := time.Now()
	if existing == nil {
		object.Id = upgradeRecordID
		return u.dao.Create(ctx, object)
	}

	// 升级流程进行中：检查器只刷新检查时间，绝不覆盖状态与目标版本。
	// object.Status 为 1/2 时同样只刷新时间，避免调用方持过期快照把已完成状态写回待升级。
	if existing.Status == model.UpgradeStatusPending || existing.Status == model.UpgradeStatusUpgrading ||
		object.Status == model.UpgradeStatusPending || object.Status == model.UpgradeStatusUpgrading {
		_, err = u.dao.Update(ctx, upgradeRecordID, map[string]interface{}{
			"last_checked_at": object.LastCheckedAt,
		}, db.WithUpgradeStatus(upgradeFlowStatuses...))
		return err
	}

	updates := map[string]interface{}{
		"current_version":   object.CurrentVersion,
		"latest_version":    object.LatestVersion,
		"image":             object.Image,
		"status":            object.Status,
		"upgrade_available": object.UpgradeAvailable,
		"last_error":        object.LastError,
		"last_checked_at":   object.LastCheckedAt,
		"gmt_modified":      now,
	}
	// 失败态发现更新的版本时重新回到未升级：清空上一轮 upgrade_id 与错误信息
	if existing.Status == model.UpgradeStatusFailed && object.Status == model.UpgradeStatusNotUpgraded {
		updates["upgrade_id"] = ""
		updates["last_error"] = ""
	}
	_, err = u.dao.Update(ctx, upgradeRecordID, updates, db.WithUpgradeStatusNotIn(upgradeFlowStatuses...))
	return err
}

func (u *upgrade) SaveCheckError(ctx context.Context, message string) error {
	_, err := u.dao.Update(ctx, upgradeRecordID, map[string]interface{}{
		"last_error":   message,
		"gmt_modified": time.Now(),
	}, db.WithUpgradeStatusNotIn(upgradeFlowStatuses...))
	return err
}

func (u *upgrade) MarkPending(ctx context.Context) (bool, error) {
	affected, err := u.dao.Update(ctx, upgradeRecordID, map[string]interface{}{
		"status":       model.UpgradeStatusPending,
		"last_error":   "",
		"upgrade_id":   "",
		"gmt_modified": time.Now(),
	},
		db.WithUpgradeStatus(model.UpgradeStatusNotUpgraded, model.UpgradeStatusFailed),
		db.WithUpgradeAvailable(true),
		db.WithUpgradeTargetReady(),
	)
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}

func (u *upgrade) Claim(ctx context.Context, upgradeID string) (*model.UpgradeRecord, bool, error) {
	affected, err := u.dao.Update(ctx, upgradeRecordID, map[string]interface{}{
		"status":       model.UpgradeStatusUpgrading,
		"upgrade_id":   upgradeID,
		"last_error":   "",
		"gmt_modified": time.Now(),
	}, db.WithUpgradeStatus(model.UpgradeStatusPending))
	if err != nil {
		return nil, false, err
	}
	if affected != 1 {
		return nil, false, nil
	}

	record, err := u.dao.Get(ctx, upgradeRecordID)
	if err != nil {
		return nil, false, err
	}
	if record == nil {
		return nil, false, nil
	}
	return record, true, nil
}

func (u *upgrade) MarkCompleted(ctx context.Context, upgradeID, version string) (bool, error) {
	affected, err := u.dao.Update(ctx, upgradeRecordID, map[string]interface{}{
		"status":            model.UpgradeStatusCompleted,
		"current_version":   version,
		"upgrade_available": false,
		"last_error":        "",
		"gmt_modified":      time.Now(),
	},
		db.WithUpgradeStatus(model.UpgradeStatusUpgrading),
		db.WithUpgradeID(upgradeID),
		db.WithUpgradeLatestVersion(version),
	)
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}

func (u *upgrade) MarkFailed(ctx context.Context, upgradeID, message string) (bool, error) {
	affected, err := u.dao.Update(ctx, upgradeRecordID, map[string]interface{}{
		"status":       model.UpgradeStatusFailed,
		"last_error":   message,
		"gmt_modified": time.Now(),
	},
		db.WithUpgradeStatus(model.UpgradeStatusUpgrading, model.UpgradeStatusCompleted),
		db.WithUpgradeID(upgradeID),
	)
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}

func (u *upgrade) ResetStale(ctx context.Context, upgradeID, message string) (bool, error) {
	affected, err := u.dao.Update(ctx, upgradeRecordID, map[string]interface{}{
		"status":       model.UpgradeStatusFailed,
		"last_error":   message,
		"gmt_modified": time.Now(),
	},
		db.WithUpgradeStatus(model.UpgradeStatusUpgrading),
		db.WithUpgradeID(upgradeID),
	)
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}
