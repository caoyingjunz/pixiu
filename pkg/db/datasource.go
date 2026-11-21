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
	"fmt"
	"strconv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"k8s.io/apimachinery/pkg/selection"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type DatasourceInterface interface {
	Create(ctx context.Context, object *model.Datasource) (*model.Datasource, error)
	Update(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (*model.Datasource, error)
	List(ctx context.Context, opts ...Options) ([]model.Datasource, error)
	Count(ctx context.Context, opts ...Options) (int64, error)
}

type datasource struct {
	db *gorm.DB
}

func newDatasource(db *gorm.DB) DatasourceInterface {
	return &datasource{db}
}

func (l *datasource) Create(ctx context.Context, object *model.Datasource) (*model.Datasource, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now
	if err := l.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (l *datasource) Update(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1

	f := l.db.WithContext(ctx).Model(&model.Datasource{}).Where("id = ? and resource_version = ?", id, resourceVersion).Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}
	return nil
}

func (l *datasource) Delete(ctx context.Context, id int64) error {
	f := l.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Datasource{})
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}
	return nil
}

func (l *datasource) Get(ctx context.Context, id int64) (*model.Datasource, error) {
	var datasource model.Datasource
	if err := l.db.WithContext(ctx).Where("id = ?", id).First(&datasource).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &datasource, nil
}

func (l *datasource) List(ctx context.Context, opts ...Options) ([]model.Datasource, error) {
	var datasources []model.Datasource
	tx := l.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Find(&datasources).Error; err != nil {
		return nil, err
	}
	return datasources, nil
}

func (l *datasource) Count(ctx context.Context, opts ...Options) (int64, error) {
	var total int64
	tx := l.db.WithContext(ctx).Model(&model.Datasource{})
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

type DatasourceLabelRequirement struct {
	Key      string
	Operator selection.Operator
	Values   []string
}

func DatasourceLabelFilter(requirements []DatasourceLabelRequirement) (Options, error) {
	conditions := make([]clause.Expression, 0, len(requirements))
	for _, req := range requirements {
		// Quote the complete key and compare values without case folding.
		path := "$." + strconv.Quote(req.Key)
		const field = "JSON_EXTRACT(`labels`, ?)"
		const text = "CAST(JSON_UNQUOTE(" + field + ") AS BINARY)"
		var condition clause.Expr
		switch req.Operator {
		case selection.Equals, selection.DoubleEquals, selection.In:
			condition = clause.Expr{SQL: text + " IN ?", Vars: []any{path, req.Values}}
		case selection.NotEquals, selection.NotIn:
			condition = clause.Expr{SQL: "(" + field + " IS NULL OR " + text + " NOT IN ?)", Vars: []any{path, path, req.Values}}
		case selection.Exists:
			condition = clause.Expr{SQL: field + " IS NOT NULL", Vars: []any{path}}
		case selection.DoesNotExist:
			condition = clause.Expr{SQL: field + " IS NULL", Vars: []any{path}}
		default:
			return nil, fmt.Errorf("unsupported label selector operator %q", req.Operator)
		}
		conditions = append(conditions, condition)
	}

	filter := clause.And(conditions...)
	return func(tx *gorm.DB) *gorm.DB { return tx.Where(filter) }, nil
}
