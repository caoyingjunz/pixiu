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

package datasource

import (
	"context"
	"fmt"
	"net/http"

	"k8s.io/klog/v2"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type Getter interface {
	Datasource() Interface
}

type Interface interface {
	Create(ctx context.Context, req *types.CreateDatasourceRequest) error
	Update(ctx context.Context, req *types.UpdateDatasourceRequest) error
	Delete(ctx context.Context, datasourceId int64) error
	Get(ctx context.Context, datasourceId int64) (*types.Datasource, error)
	List(ctx context.Context, listOption types.ListOptions) (interface{}, error)
}

type controller struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &controller{cc: cfg, factory: f}
}

func (c *controller) preCreate(ctx context.Context, req *types.CreateDatasourceRequest) error {
	datasources, err := c.factory.Datasource().List(
		ctx,
		db.WithClusterName(req.ClusterName),
		db.WithDatasourceType(req.Type),
	)
	if err != nil {
		klog.Errorf(
			"failed to list datasources for validation cluster=%s type=%d sub_type=%s name=%s: %v",
			req.ClusterName,
			req.Type,
			req.SubType,
			req.Name,
			err,
		)
		return apierrors.ErrServerInternal
	}

	for _, datasource := range datasources {
		if req.IsDefault && datasource.IsDefault {
			return apierrors.NewError(
				fmt.Errorf("default datasource already exists for cluster=%s type=%d, please unset the current default first",
					req.ClusterName, req.Type),
				http.StatusConflict,
			)
		}

		if datasource.Name == req.Name {
			return apierrors.NewError(
				fmt.Errorf("datasource already exists: cluster=%s type=%d name=%s",
					req.ClusterName, req.Type, req.Name),
				http.StatusConflict,
			)
		}
	}

	return nil
}

func (c *controller) Create(ctx context.Context, req *types.CreateDatasourceRequest) error {
	if err := c.preCreate(ctx, req); err != nil {
		return err
	}

	// 对配置进行简化，移除不必要的配置
	req.Config.Clean(req.Type)
	cfg, err := req.Config.Marshal()
	if err != nil {
		return apierrors.NewError(fmt.Errorf("invalid datasource config: %v", err), http.StatusBadRequest)
	}

	_, err = c.factory.Datasource().Create(ctx, &model.Datasource{
		ClusterName: req.ClusterName,
		Name:        req.Name,
		Type:        req.Type,
		SubType:     req.SubType,
		Config:      cfg,
		IsDefault:   req.IsDefault,
		Description: req.Description,
	})
	if err != nil {
		klog.Errorf("failed to create datasource %s: %v", req.Name, err)
		return apierrors.ErrServerInternal
	}

	return nil
}

func (c *controller) Update(ctx context.Context, req *types.UpdateDatasourceRequest) error {
	old, err := c.factory.Datasource().Get(ctx, req.Id)
	if err != nil {
		klog.Errorf("failed to get datasource %d: %v", req.Id, err)
		return apierrors.ErrServerInternal
	}
	if old == nil {
		return apierrors.NewError(fmt.Errorf("datasource not found"), http.StatusNotFound)
	}

	updates := make(map[string]interface{})

	req.Config.Clean(req.Type)
	cfg, err := req.Config.Marshal()
	if err != nil {
		return err
	}
	if cfg != old.Config {
		updates["config"] = cfg
	}

	if req.Name != old.Name {
		updates["name"] = req.Name
	}
	if req.ClusterName != old.ClusterName {
		updates["cluster_name"] = req.ClusterName
	}
	if req.Type != old.Type {
		updates["type"] = req.Type
	}
	if req.SubType != old.SubType {
		updates["sub_type"] = req.SubType
	}
	if req.IsDefault != old.IsDefault {
		updates["is_default"] = req.IsDefault
	}
	if req.Description != old.Description {
		updates["description"] = req.Description
	}

	if len(updates) == 0 {
		return nil
	}
	if err = c.factory.Datasource().Update(ctx, req.Id, req.ResourceVersion, updates); err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return apierrors.NewError(
				fmt.Errorf("datasource not found or resource version conflict"),
				http.StatusConflict,
			)
		}
		klog.Errorf("failed to update datasource %d: %v", req.Id, err)
		return apierrors.ErrServerInternal
	}

	return nil
}

func (c *controller) Delete(ctx context.Context, datasourceId int64) error {
	if err := c.factory.Datasource().Delete(ctx, datasourceId); err != nil {
		klog.Errorf("failed to delete datasource %d: %v", datasourceId, err)
		return apierrors.ErrServerInternal
	}
	return nil
}

func (c *controller) Get(ctx context.Context, datasourceId int64) (*types.Datasource, error) {
	object, err := c.factory.Datasource().Get(ctx, datasourceId)
	if err != nil {
		klog.Errorf("failed to get datasource(%d): %v", datasourceId, err)
		return nil, apierrors.ErrServerInternal
	}
	if object == nil {
		return nil, apierrors.NewError(fmt.Errorf("datasource not found"), http.StatusNotFound)
	}
	return modelToType(object)
}

func (c *controller) List(ctx context.Context, listOption types.ListOptions) (interface{}, error) {
	listOption.SetDefaultPageOption()

	pageResult := types.PageResult{
		PageRequest: types.PageRequest{
			Page:  listOption.Page,
			Limit: listOption.Limit,
		},
	}

	opts := []db.Options{
		db.WithNameLike(listOption.NameSelector),
		db.WithClusterName(listOption.ClusterName),
	}
	if listOption.DatasourceType != nil {
		opts = append(opts, db.WithDatasourceType(*listOption.DatasourceType))
	}

	var err error
	pageResult.Total, err = c.factory.Datasource().Count(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to count dataSources: %v", err)
		pageResult.Message = err.Error()
	}

	offset := (listOption.Page - 1) * listOption.Limit
	opts = append(opts, []db.Options{
		db.WithModifyOrderByDesc(),
		db.WithOffset(offset),
		db.WithLimit(listOption.Limit),
	}...)

	objects, err := c.factory.Datasource().List(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to list datasources: %v", err)
		pageResult.Message = err.Error()
		return nil, apierrors.ErrServerInternal
	}

	items := make([]types.Datasource, 0, len(objects))
	for i := range objects {
		t, convErr := modelToType(&objects[i])
		if convErr != nil {
			return nil, apierrors.ErrServerInternal
		}
		items = append(items, *t)
	}
	pageResult.Items = items

	return pageResult, nil
}

func modelToType(object *model.Datasource) (*types.Datasource, error) {
	var cfg types.DatasourceConfig
	if err := cfg.Unmarshal(object.Config); err != nil {
		return nil, err
	}
	return &types.Datasource{
		PixiuMeta: types.PixiuMeta{
			Id:              object.Id,
			ResourceVersion: object.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   object.GmtCreate,
			GmtModified: object.GmtModified,
		},
		ClusterName: object.ClusterName,
		Name:        object.Name,
		Type:        object.Type,
		SubType:     object.SubType,
		Config:      cfg,
		IsDefault:   object.IsDefault,
		Description: object.Description,
	}, nil
}
