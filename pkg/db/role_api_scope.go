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
	"context"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

type RoleAPIScopeInterface interface {
	ListByRoleId(ctx context.Context, roleId int64) ([]model.RoleAPIScope, error)
	ReplaceByRoleId(ctx context.Context, roleId int64, scopes []model.RoleAPIScope) error
	Add(ctx context.Context, roleId int64, scopes []model.RoleAPIScope) error
	Remove(ctx context.Context, roleId int64, scopes []model.RoleAPIScope) error
}

type roleAPIScope struct {
	db *gorm.DB
}

func normalizeScope(s *model.RoleAPIScope) {
	s.Cluster = strings.TrimSpace(s.Cluster)
	s.Namespace = strings.TrimSpace(s.Namespace)
	s.ResourceName = strings.TrimSpace(s.ResourceName)
	if s.ResourceName == "" {
		s.ResourceName = "*"
	}
}

func (r *roleAPIScope) ListByRoleId(ctx context.Context, roleId int64) ([]model.RoleAPIScope, error) {
	var scopes []model.RoleAPIScope
	if err := r.db.WithContext(ctx).Where("role_id = ?", roleId).Find(&scopes).Error; err != nil {
		return nil, err
	}
	return scopes, nil
}

func prepareScopeRecords(roleId int64, scopes []model.RoleAPIScope) []model.RoleAPIScope {
	now := time.Now()
	records := make([]model.RoleAPIScope, 0, len(scopes))
	seen := make(map[string]struct{}, len(scopes))
	for i := range scopes {
		item := scopes[i]
		item.RoleId = roleId
		normalizeScope(&item)
		if item.APIId <= 0 || item.Cluster == "" || item.Namespace == "" {
			continue
		}
		key := scopeKey(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		item.GmtCreate = now
		item.GmtModified = now
		records = append(records, item)
	}
	return records
}

func (r *roleAPIScope) ReplaceByRoleId(ctx context.Context, roleId int64, scopes []model.RoleAPIScope) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleId).Delete(&model.RoleAPIScope{}).Error; err != nil {
			return err
		}
		records := prepareScopeRecords(roleId, scopes)
		if len(records) == 0 {
			return nil
		}
		return tx.Create(&records).Error
	})
}

func (r *roleAPIScope) Add(ctx context.Context, roleId int64, scopes []model.RoleAPIScope) error {
	records := prepareScopeRecords(roleId, scopes)
	if len(records) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&records).Error
}

func (r *roleAPIScope) Remove(ctx context.Context, roleId int64, scopes []model.RoleAPIScope) error {
	if len(scopes) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i := range scopes {
			item := scopes[i]
			normalizeScope(&item)
			if err := tx.Where(
				"role_id = ? AND api_id = ? AND cluster = ? AND namespace = ? AND resource_name = ?",
				roleId, item.APIId, item.Cluster, item.Namespace, item.ResourceName,
			).Delete(&model.RoleAPIScope{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func scopeKey(s model.RoleAPIScope) string {
	return strings.Join([]string{
		strconv.FormatInt(s.APIId, 10),
		s.Cluster,
		s.Namespace,
		s.ResourceName,
	}, "|")
}

func newRoleAPIScope(db *gorm.DB) *roleAPIScope {
	return &roleAPIScope{db: db}
}
