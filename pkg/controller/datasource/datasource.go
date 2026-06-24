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
)

type Getter interface {
	Datasource() Interface
}

type Interface interface {
	Create(ctx context.Context, req *types.CreateDatasourceRequest) error
	Update(ctx context.Context, clusterName string, datasourceType model.DatasourceType, datasourceId int64, req *types.UpdateDatasourceRequest) error
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
	if req.IsDefault {
		defaultCount, err := c.factory.Datasource().Count(
			ctx,
			db.WithClusterName(req.ClusterName),
			db.WithDatasourceType(req.Type),
			db.WithDatasourceIsDefault(true),
		)
		if err != nil {
			klog.Errorf(
				"failed to check default datasource cluster=%s type=%d sub_type=%s name=%s: %v",
				req.ClusterName,
				req.Type,
				req.SubType,
				req.Name,
				err,
			)
			return apierrors.ErrServerInternal
		}
		if defaultCount > 0 {
			return apierrors.NewError(
				fmt.Errorf("default datasource already exists for cluster=%s type=%d, please unset the current default first",
					req.ClusterName, req.Type),
				http.StatusConflict,
			)
		}
	}

	count, err := c.factory.Datasource().Count(
		ctx,
		db.WithClusterName(req.ClusterName),
		db.WithDatasourceType(req.Type),
		db.WithDatasourceSubType(req.SubType),
		db.WithName(req.Name),
	)
	if err != nil {
		klog.Errorf(
			"failed to check datasource duplicate cluster=%s type=%d sub_type=%s name=%s: %v",
			req.ClusterName,
			req.Type,
			req.SubType,
			req.Name,
			err,
		)
		return apierrors.ErrServerInternal
	}
	if count > 0 {
		return apierrors.NewError(
			fmt.Errorf("datasource already exists: cluster=%s type=%d sub_type=%s name=%s",
				req.ClusterName, req.Type, req.SubType, req.Name),
			http.StatusConflict,
		)
	}
	return nil
}

func (c *controller) Create(ctx context.Context, req *types.CreateDatasourceRequest) error {
	if err := c.preCreate(ctx, req); err != nil {
		return err
	}

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

func (c *controller) Update(ctx context.Context, clusterName string, datasourceType model.DatasourceType, datasourceId int64, req *types.UpdateDatasourceRequest) error {
	//cluster, err := c.mustGetCluster(ctx, clusterName)
	//if err != nil {
	//	return err
	//}
	//object, err := c.mustGetDatasource(ctx, cluster.Name, datasourceType, datasourceId)
	//if err != nil {
	//	return err
	//}
	//
	//nextSubType := object.SubType
	//if req.SubType != nil {
	//	nextSubType = *req.SubType
	//}
	//if err = validateDatasourceType(datasourceType, nextSubType); err != nil {
	//	return apierrors.NewError(err, http.StatusBadRequest)
	//}
	//
	//updates := make(map[string]interface{})
	//if req.Name != nil {
	//	updates["name"] = *req.Name
	//}
	//if req.SubType != nil {
	//	updates["sub_type"] = *req.SubType
	//}
	//if req.URL != nil {
	//	updates["url"] = *req.URL
	//}
	//if req.Config != nil {
	//	datasourceConfig, marshalErr := req.Config.Marshal()
	//	if marshalErr != nil {
	//		return apierrors.NewError(fmt.Errorf("invalid datasource config: %v", marshalErr), http.StatusBadRequest)
	//	}
	//	updates["config"] = datasourceConfig
	//}
	//if req.Description != nil {
	//	updates["description"] = *req.Description
	//}
	//if req.IsDefault != nil {
	//	updates["is_default"] = *req.IsDefault
	//}
	//if len(updates) == 0 {
	//	return apierrors.ErrInvalidRequest
	//}
	//
	//if err = c.factory.Datasource().Update(ctx, datasourceId, *req.ResourceVersion, updates); err != nil {
	//	klog.Errorf("failed to update datasource %d: %v", datasourceId, err)
	//	return apierrors.ErrServerInternal
	//}

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

	opts := buildDatasourceFilterOpts(listOption)

	total, err := c.factory.Datasource().Count(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to count datasources: %v", err)
		pageResult.Message = err.Error()
		return nil, apierrors.ErrServerInternal
	}
	pageResult.Total = total

	offset := (listOption.Page - 1) * listOption.Limit
	opts = append(opts, db.WithOrderByDesc(), db.WithOffset(offset), db.WithLimit(listOption.Limit))

	objects, err := c.factory.Datasource().List(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to list datasources: %v", err)
		pageResult.Message = err.Error()
		return nil, apierrors.ErrServerInternal
	}

	result := make([]types.Datasource, 0, len(objects))
	for i := range objects {
		t, convErr := modelToType(&objects[i])
		if convErr != nil {
			return nil, apierrors.ErrServerInternal
		}
		result = append(result, *t)
	}
	pageResult.Items = result
	return pageResult, nil
}

func buildDatasourceFilterOpts(opt types.ListOptions) []db.Options {
	opts := make([]db.Options, 0, 4)
	if opt.NameSelector != "" {
		opts = append(opts, db.WithNameLike(opt.NameSelector))
	}
	return opts
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

func validateDatasourceType(datasourceType model.DatasourceType, subType model.DatasourceSubType) error {
	switch datasourceType {
	case model.DatasourceTypeLog:
		if subType != model.DatasourceSubTypeLoki && subType != model.DatasourceSubTypeES {
			return fmt.Errorf("invalid datasource sub_type %q for type %d", subType, datasourceType)
		}
	case model.DatasourceTypeAlert:
		if subType != model.DatasourceSubTypePrometheus {
			return fmt.Errorf("invalid datasource sub_type %q for type %d", subType, datasourceType)
		}
	default:
		return fmt.Errorf("invalid datasource type %d", datasourceType)
	}
	return nil
}
