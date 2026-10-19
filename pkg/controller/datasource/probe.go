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

package datasource

import (
	"context"
	"fmt"
	"net/http"

	"k8s.io/klog/v2"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	controllerutil "github.com/caoyingjunz/pixiu/pkg/controller/util"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

// LoadDatasourceConfig 统一加载数据源基础信息并做类型、归属与配置反序列化校验。
// validate 由调用方提供，用于校验具体数据源类型/子类型/External 约束；
// 本函数不做管理员校验，PingAdhoc 的 admin 校验由调用方在入口完成。
func LoadDatasourceConfig(
	ctx context.Context,
	factory db.ShareDaoFactory,
	datasourceId int64,
	validate func(*model.Datasource) error,
) (*model.Datasource, *types.DatasourceConfig, error) {
	return loadDatasourceConfig(ctx, factory, datasourceId, validate, apierrors.ErrServerInternal)
}

// LoadDatasourceConfigForAdhoc 与 LoadDatasourceConfig 相同，但配置反序列化失败时
// 返回 400 invalid <kind> datasource config，用于临时探测接口。
func LoadDatasourceConfigForAdhoc(
	ctx context.Context,
	factory db.ShareDaoFactory,
	datasourceId int64,
	validate func(*model.Datasource) error,
	kind string,
) (*model.Datasource, *types.DatasourceConfig, error) {
	return loadDatasourceConfig(ctx, factory, datasourceId, validate,
		apierrors.NewError(fmt.Errorf("invalid %s datasource config", kind), http.StatusBadRequest))
}

func loadDatasourceConfig(
	ctx context.Context,
	factory db.ShareDaoFactory,
	datasourceId int64,
	validate func(*model.Datasource) error,
	onUnmarshalErr error,
) (*model.Datasource, *types.DatasourceConfig, error) {
	object, err := factory.Datasource().Get(ctx, datasourceId)
	if err != nil {
		klog.Errorf("failed to get datasource(%d): %v", datasourceId, err)
		return nil, nil, apierrors.ErrServerInternal
	}
	if object == nil {
		return nil, nil, apierrors.NewError(fmt.Errorf("datasource not found"), http.StatusNotFound)
	}
	if validate != nil {
		if err = validate(object); err != nil {
			return nil, nil, err
		}
	}
	// 归属校验：root/owner 放行，其余须命中角色 scope，与 datasource controller 的 Get 完全一致
	if err = controllerutil.CheckResourceAccess(ctx, factory, object.UserId, types.ResourceTypeDatasource, datasourceId); err != nil {
		klog.Warningf("datasource(%d) access denied: %v", datasourceId, err)
		return nil, nil, err
	}

	var cfg types.DatasourceConfig
	if err = cfg.Unmarshal(object.Config); err != nil {
		klog.Errorf("failed to unmarshal datasource(%d) config: %v", datasourceId, err)
		return nil, nil, onUnmarshalErr
	}
	return object, &cfg, nil
}
