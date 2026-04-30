/*
Copyright 2024 The Pixiu Authors.

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
	"time"

	"gorm.io/gorm"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

type Options func(*gorm.DB) *gorm.DB

func WithOrderByASC() Options {
	return func(tx *gorm.DB) *gorm.DB {
		return tx.Order("id ASC")
	}
}

func WithOrderByDesc() Options {
	return func(tx *gorm.DB) *gorm.DB {
		return tx.Order("id DESC")
	}
}

func WithOffset(offset int) Options {
	return func(tx *gorm.DB) *gorm.DB {
		return tx.Offset(offset)
	}
}

func WithCreatedBefore(t time.Time) Options {
	return func(tx *gorm.DB) *gorm.DB {
		return tx.Where("gmt_create < ?", t)
	}
}

func WithLimit(limit int) Options {
	return func(tx *gorm.DB) *gorm.DB {
		if limit == 0 {
			// `LIMIT 0` statement will return 0 rows, it's meaningless.
			return tx
		}
		return tx.Limit(limit)
	}
}

func WithIDIn(ids ...int64) Options {
	return func(tx *gorm.DB) *gorm.DB {
		// e.g. `WHERE id IN (1, 2, 3)`
		return tx.Where("id IN ?", ids)
	}
}

func WithAliasNameLike(name string) Options {
	return func(tx *gorm.DB) *gorm.DB {
		if name == "" {
			return tx
		}
		return tx.Where("alias_name like ?", "%"+name+"%")
	}
}

func WithClusterStatus(status int) Options {
	return func(tx *gorm.DB) *gorm.DB {
		return tx.Where("status = ?", status)
	}
}

func WithUserNameLike(name string) Options {
	return func(tx *gorm.DB) *gorm.DB {
		if name == "" {
			return tx
		}
		return tx.Where("name like ?", "%"+name+"%")
	}
}

func WithUserPhoneLike(phone string) Options {
	return func(tx *gorm.DB) *gorm.DB {
		if phone == "" {
			return tx
		}
		return tx.Where("phone like ?", "%"+phone+"%")
	}
}

func WithUserEmailLike(email string) Options {
	return func(tx *gorm.DB) *gorm.DB {
		if email == "" {
			return tx
		}
		return tx.Where("email like ?", "%"+email+"%")
	}
}

func WithUserStatus(status int) Options {
	return func(tx *gorm.DB) *gorm.DB {
		return tx.Where("status = ?", status)
	}
}

func WithAuditOperatorLike(operator string) Options {
	return func(tx *gorm.DB) *gorm.DB {
		if operator == "" {
			return tx
		}
		return tx.Where("operator like ?", "%"+operator+"%")
	}
}

func WithAuditAction(action string) Options {
	return func(tx *gorm.DB) *gorm.DB {
		if action == "" {
			return tx
		}
		return tx.Where("action = ?", action)
	}
}

func WithAuditObjectType(ot string) Options {
	return func(tx *gorm.DB) *gorm.DB {
		if ot == "" {
			return tx
		}
		return tx.Where("resource_type = ?", ot)
	}
}

func WithAuditStatus(status uint8) Options {
	return func(tx *gorm.DB) *gorm.DB {
		return tx.Where("status = ?", status)
	}
}

func WithAuditCreatedAfter(t time.Time) Options {
	return func(tx *gorm.DB) *gorm.DB {
		if t.IsZero() {
			return tx
		}
		return tx.Where("gmt_create >= ?", t)
	}
}

func WithAuditCluster(cluster string) Options {
	return func(tx *gorm.DB) *gorm.DB {
		return tx.Where("cluster = ?", cluster)
	}
}

func WithPlanNameLike(name string) Options {
	return func(tx *gorm.DB) *gorm.DB {
		if name == "" {
			return tx
		}
		return tx.Where("name like ?", "%"+name+"%")
	}
}

func WithNameLike(name string) Options {
	return func(tx *gorm.DB) *gorm.DB {
		if name == "" {
			return tx
		}
		return tx.Where("name like ?", "%"+name+"%")
	}
}

func WithPlan(pid int64) Options {
	return func(tx *gorm.DB) *gorm.DB {
		if pid == 0 {
			return tx
		}
		return tx.Where("plan_id = ?", pid)
	}
}

func WithStatus(status model.AgentStatus) Options {
	return func(tx *gorm.DB) *gorm.DB {
		return tx.Where("status = ?", status)
	}
}

func WithUserId(userId int64) Options {
	return func(tx *gorm.DB) *gorm.DB {
		return tx.Where("user_id = ?", userId)
	}
}
