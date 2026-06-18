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

package logdatasource

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"k8s.io/klog/v2"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type Getter interface {
	LogDatasource() Interface
}

type Interface interface {
	Create(ctx context.Context, clusterName string, req *types.CreateClusterLogDatasourceRequest) error
	Update(ctx context.Context, clusterName string, datasourceId int64, req *types.UpdateClusterLogDatasourceRequest) error
	Delete(ctx context.Context, clusterName string, datasourceId int64) error
	Get(ctx context.Context, clusterName string, datasourceId int64) (*types.ClusterLogDatasource, error)
	List(ctx context.Context, clusterName string) ([]types.ClusterLogDatasource, error)
	GetDefault(ctx context.Context, clusterName string) (*types.ClusterLogDatasource, error)
	SetDefault(ctx context.Context, clusterName string, datasourceId int64) error
}

type controller struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &controller{cc: cfg, factory: f}
}

func (c *controller) Create(ctx context.Context, clusterName string, req *types.CreateClusterLogDatasourceRequest) error {
	cluster, err := c.mustGetCluster(ctx, clusterName)
	if err != nil {
		return err
	}
	headers, err := marshalHeaders(req.Headers)
	if err != nil {
		return apierrors.NewError(fmt.Errorf("invalid datasource headers: %v", err), http.StatusBadRequest)
	}

	object := &model.ClusterLogDatasource{
		ClusterName: cluster.Name,
		Name:        req.Name,
		Type:        req.Type,
		URL:         req.URL,
		Username:    req.Username,
		Password:    req.Password,
		Headers:     headers,
		IsDefault:   req.IsDefault,
		Description: req.Description,
	}
	created, err := c.factory.LogDatasource().Create(ctx, object)
	if err != nil {
		klog.Errorf("failed to create log datasource %s: %v", req.Name, err)
		return apierrors.ErrServerInternal
	}
	if created.IsDefault {
		if err = c.factory.LogDatasource().UpdateDefaultByCluster(ctx, cluster.Name, created.Id); err != nil {
			klog.Errorf("failed to set default log datasource %d: %v", created.Id, err)
			return apierrors.ErrServerInternal
		}
	}
	return nil
}

func (c *controller) Update(ctx context.Context, clusterName string, datasourceId int64, req *types.UpdateClusterLogDatasourceRequest) error {
	cluster, err := c.mustGetCluster(ctx, clusterName)
	if err != nil {
		return err
	}
	object, err := c.mustGetDatasource(ctx, cluster.Name, datasourceId)
	if err != nil {
		return err
	}

	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Type != nil {
		updates["type"] = *req.Type
	}
	if req.URL != nil {
		updates["url"] = *req.URL
	}
	if req.Username != nil {
		updates["username"] = *req.Username
	}
	if req.Password != nil {
		updates["password"] = *req.Password
	}
	if req.Headers != nil {
		headers, marshalErr := marshalHeaders(*req.Headers)
		if marshalErr != nil {
			return apierrors.NewError(fmt.Errorf("invalid datasource headers: %v", marshalErr), http.StatusBadRequest)
		}
		updates["headers"] = headers
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.IsDefault != nil {
		updates["is_default"] = *req.IsDefault
	}
	if len(updates) == 0 {
		return apierrors.ErrInvalidRequest
	}

	if err = c.factory.LogDatasource().Update(ctx, datasourceId, *req.ResourceVersion, updates); err != nil {
		klog.Errorf("failed to update log datasource %d: %v", datasourceId, err)
		return apierrors.ErrServerInternal
	}
	if req.IsDefault != nil && *req.IsDefault {
		if err = c.factory.LogDatasource().UpdateDefaultByCluster(ctx, cluster.Name, object.Id); err != nil {
			klog.Errorf("failed to set default log datasource %d: %v", object.Id, err)
			return apierrors.ErrServerInternal
		}
	}
	return nil
}

func (c *controller) Delete(ctx context.Context, clusterName string, datasourceId int64) error {
	cluster, err := c.mustGetCluster(ctx, clusterName)
	if err != nil {
		return err
	}
	if _, err := c.mustGetDatasource(ctx, cluster.Name, datasourceId); err != nil {
		return err
	}
	if err := c.factory.LogDatasource().Delete(ctx, datasourceId); err != nil {
		klog.Errorf("failed to delete log datasource %d: %v", datasourceId, err)
		return apierrors.ErrServerInternal
	}
	return nil
}

func (c *controller) Get(ctx context.Context, clusterName string, datasourceId int64) (*types.ClusterLogDatasource, error) {
	cluster, err := c.mustGetCluster(ctx, clusterName)
	if err != nil {
		return nil, err
	}
	object, err := c.mustGetDatasource(ctx, cluster.Name, datasourceId)
	if err != nil {
		return nil, err
	}
	return modelToType(object)
}

func (c *controller) List(ctx context.Context, clusterName string) ([]types.ClusterLogDatasource, error) {
	cluster, err := c.mustGetCluster(ctx, clusterName)
	if err != nil {
		return nil, err
	}
	objects, err := c.factory.LogDatasource().ListByCluster(ctx, cluster.Name)
	if err != nil {
		klog.Errorf("failed to list log datasources for cluster %s: %v", clusterName, err)
		return nil, apierrors.ErrServerInternal
	}

	result := make([]types.ClusterLogDatasource, 0, len(objects))
	for i := range objects {
		t, convErr := modelToType(&objects[i])
		if convErr != nil {
			return nil, apierrors.ErrServerInternal
		}
		result = append(result, *t)
	}
	return result, nil
}

func (c *controller) GetDefault(ctx context.Context, clusterName string) (*types.ClusterLogDatasource, error) {
	cluster, err := c.mustGetCluster(ctx, clusterName)
	if err != nil {
		return nil, err
	}
	object, err := c.factory.LogDatasource().GetDefaultByCluster(ctx, cluster.Name)
	if err != nil {
		klog.Errorf("failed to get default log datasource for cluster %s: %v", clusterName, err)
		return nil, apierrors.ErrServerInternal
	}
	if object == nil {
		return nil, apierrors.NewError(fmt.Errorf("no default log datasource found, please add one first"), http.StatusNotFound)
	}
	return modelToType(object)
}

func (c *controller) SetDefault(ctx context.Context, clusterName string, datasourceId int64) error {
	cluster, err := c.mustGetCluster(ctx, clusterName)
	if err != nil {
		return err
	}
	if _, err := c.mustGetDatasource(ctx, cluster.Name, datasourceId); err != nil {
		return err
	}
	if err := c.factory.LogDatasource().UpdateDefaultByCluster(ctx, cluster.Name, datasourceId); err != nil {
		klog.Errorf("failed to set default log datasource %d: %v", datasourceId, err)
		return apierrors.ErrServerInternal
	}
	return nil
}

func (c *controller) mustGetCluster(ctx context.Context, clusterName string) (*model.Cluster, error) {
	object, err := c.factory.Cluster().GetClusterByName(ctx, clusterName)
	if err != nil {
		klog.Errorf("failed to get cluster(%s): %v", clusterName, err)
		return nil, apierrors.ErrServerInternal
	}
	if object == nil {
		return nil, apierrors.ErrClusterNotFound
	}
	return object, nil
}

func (c *controller) mustGetDatasource(ctx context.Context, clusterName string, datasourceId int64) (*model.ClusterLogDatasource, error) {
	object, err := c.factory.LogDatasource().Get(ctx, datasourceId)
	if err != nil {
		klog.Errorf("failed to get log datasource(%d): %v", datasourceId, err)
		return nil, apierrors.ErrServerInternal
	}
	if object == nil {
		return nil, apierrors.NewError(fmt.Errorf("log datasource not found"), http.StatusNotFound)
	}
	if object.ClusterName != clusterName {
		return nil, apierrors.NewError(fmt.Errorf("log datasource not found"), http.StatusNotFound)
	}
	return object, nil
}

func marshalHeaders(headers []types.HTTPHeader) (string, error) {
	if len(headers) == 0 {
		return "[]", nil
	}
	data, err := json.Marshal(headers)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func unmarshalHeaders(raw string) ([]types.HTTPHeader, error) {
	if strings.TrimSpace(raw) == "" {
		return []types.HTTPHeader{}, nil
	}
	var headers []types.HTTPHeader
	if err := json.Unmarshal([]byte(raw), &headers); err != nil {
		return nil, err
	}
	return headers, nil
}

func modelToType(object *model.ClusterLogDatasource) (*types.ClusterLogDatasource, error) {
	headers, err := unmarshalHeaders(object.Headers)
	if err != nil {
		return nil, err
	}
	return &types.ClusterLogDatasource{
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
		URL:         object.URL,
		Username:    object.Username,
		Headers:     headers,
		HasPassword: strings.TrimSpace(object.Password) != "",
		IsDefault:   object.IsDefault,
		Description: object.Description,
	}, nil
}
