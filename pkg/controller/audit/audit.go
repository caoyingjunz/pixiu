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

package audit

import (
	"context"
	"net/http"
	"time"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	controllerutil "github.com/caoyingjunz/pixiu/pkg/controller/util"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type AuditGetter interface {
	Audit() Interface
}

type Interface interface {
	List(ctx context.Context, listOption types.ListOptions) (interface{}, error)
	ListRecent(ctx context.Context, listOption types.ListOptions) ([]types.Audit, error)
	Export(ctx context.Context, listOption types.ListOptions) (*AuditExport, error)
}

type audit struct {
	factory db.ShareDaoFactory
}

var auditModuleTypes = map[string][]string{
	"cluster":    {string(model.ObjectCluster), string(model.ObjectContainerProxy)},
	"middleware": {string(model.ObjectMySQL), string(model.ObjectPostgres), string(model.ObjectRedis), string(model.ObjectStorage), string(model.ObjectNacos), string(model.ObjectES), string(model.ObjectRabbitMQ)},
	"datasource": {string(model.ObjectDatasource)},
	"identity":   {string(model.ObjectUser), string(model.ObjectTenant), string(model.ObjectAuth)},
	"app":        {string(model.ObjectApp)},
	"backup":     {string(model.ObjectBackup)},
}

func (a *audit) List(ctx context.Context, listOption types.ListOptions) (interface{}, error) {
	listOption.SetDefaultPageOption()

	pageResult := types.PageResult{
		PageRequest: types.PageRequest{
			Page:  listOption.Page,
			Limit: listOption.Limit,
		},
	}

	filterOpts, err := a.buildFilterOpts(ctx, listOption, false)
	if err != nil {
		return nil, err
	}

	pageResult.Total, err = a.factory.Audit().Count(ctx, filterOpts...)
	if err != nil {
		klog.Errorf("failed to get audits count: %v", err)
		return nil, errors.ErrServerInternal
	}

	offset := (listOption.Page - 1) * listOption.Limit
	if pageResult.Total == 0 || int64(offset) >= pageResult.Total {
		pageResult.Items = []types.Audit{}
		return pageResult, nil
	}
	paginationOpts := append(filterOpts,
		db.WithOffset(offset),
		db.WithLimit(listOption.Limit),
		db.WithOrderByDesc(),
	)

	objects, err := a.factory.Audit().List(ctx, paginationOpts...)
	if err != nil {
		klog.Errorf("failed to get audit events: %v", err)
		return nil, errors.ErrServerInternal
	}

	items, err := a.hydrate(ctx, objects)
	if err != nil {
		return nil, err
	}
	pageResult.Items = items
	return pageResult, nil
}

func (a *audit) ListRecent(ctx context.Context, listOption types.ListOptions) ([]types.Audit, error) {
	limit := listOption.Limit
	if limit <= 0 {
		limit = 8
	}
	if limit > 20 {
		limit = 20
	}
	filterOpts, err := a.buildFilterOpts(ctx, listOption, true)
	if err != nil {
		return nil, err
	}
	// 只取最近动态渲染需要的列，减小响应体与数据库读取量
	opts := append(filterOpts, db.WithLimit(limit), db.WithOrderByDesc(), db.WithColumns(
		"id", "gmt_create", "operator", "operator_id", "status", "resource_type",
		"cluster", "resource_name", "resource_namespace", "resource_id", "operation", "resource_kind",
	))
	objects, err := a.factory.Audit().List(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to get recent audits: %v", err)
		return nil, errors.ErrServerInternal
	}
	return a.hydrate(ctx, objects)
}

func (a *audit) buildFilterOpts(ctx context.Context, opt types.ListOptions, scoped bool) ([]db.Options, error) {
	opts := []db.Options{db.WithAuditExcludeK8sReviews()}
	if opt.Operator != "" {
		opts = append(opts, db.WithAuditOperator(opt.Operator))
	}
	if opt.Action != "" {
		opts = append(opts, db.WithAuditAction(opt.Action))
	}
	if opt.Operation != "" {
		opts = append(opts, db.WithAuditOperation(opt.Operation))
	}
	if types := expandAuditTypes(opt.Module, opt.ObjectType); len(types) > 0 {
		opts = append(opts, db.WithAuditObjectTypes(types))
	}
	if opt.ClusterName != "" {
		opts = append(opts, db.WithAuditCluster(opt.ClusterName))
	}
	if opt.Status != nil {
		opts = append(opts, db.WithAuditStatus(uint8(*opt.Status)))
	}
	if opt.StartTime != "" {
		if t, err := time.Parse(time.RFC3339, opt.StartTime); err == nil {
			opts = append(opts, db.WithAuditCreatedAfter(t))
		}
	}
	if opt.EndTime != "" {
		if t, err := time.Parse(time.RFC3339, opt.EndTime); err == nil {
			opts = append(opts, db.WithCreatedBefore(t))
		}
	}

	user, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if scoped {
		platform, err := a.hasPlatformAudit(ctx, user)
		if err != nil {
			return nil, err
		}
		if !platform {
			scopeOpt, err := a.scopeOption(ctx, user)
			if err != nil {
				return nil, err
			}
			opts = append(opts, scopeOpt)
		}
	}
	return opts, nil
}

func expandAuditTypes(module, objectType string) []string {
	if objectType != "" {
		return []string{objectType}
	}
	if module == "" {
		return nil
	}
	if types, ok := auditModuleTypes[module]; ok {
		return append([]string{}, types...)
	}
	return []string{module}
}

func (a *audit) hasPlatformAudit(ctx context.Context, user *model.User) (bool, error) {
	if user.Role == model.RoleRoot {
		return true, nil
	}
	ok, err := a.factory.API().ExistsByRoleMethodPath(ctx, int64(user.Role), http.MethodGet, "/pixiu/audits")
	if err != nil {
		klog.Errorf("failed to check audit list permission for role %d: %v", user.Role, err)
		return false, errors.ErrServerInternal
	}
	return ok, nil
}

func (a *audit) scopeOption(ctx context.Context, user *model.User) (db.Options, error) {
	clusterIDs, err := controllerutil.AuthorizedResourceIDs(ctx, a.factory, types.ResourceTypeCluster)
	if err != nil {
		klog.Errorf("failed to get authorized clusters for audit scope: %v", err)
		return nil, errors.ErrServerInternal
	}
	clusters, err := a.factory.Cluster().List(ctx, db.WithUserOrResourceIDs(user.Id, clusterIDs), db.WithColumns("name"))
	if err != nil {
		klog.Errorf("failed to list clusters for audit scope: %v", err)
		return nil, errors.ErrServerInternal
	}
	clusterNames := make([]string, 0, len(clusters))
	for i := range clusters {
		if clusters[i].Name != "" {
			clusterNames = append(clusterNames, clusters[i].Name)
		}
	}

	datasourceIDs, err := controllerutil.AuthorizedResourceIDs(ctx, a.factory, types.ResourceTypeDatasource)
	if err != nil {
		klog.Errorf("failed to get authorized datasources for audit scope: %v", err)
		return nil, errors.ErrServerInternal
	}
	datasources, err := a.factory.Datasource().List(ctx, db.WithUserOrResourceIDs(user.Id, datasourceIDs), db.WithColumns("id"))
	if err != nil {
		klog.Errorf("failed to list datasources for audit scope: %v", err)
		return nil, errors.ErrServerInternal
	}
	resourceIDs := make([]int64, 0, len(datasources))
	for i := range datasources {
		resourceIDs = append(resourceIDs, datasources[i].Id)
	}
	return db.WithAuditScope(user.Id, user.Name, clusterNames, resourceIDs), nil
}

func (a *audit) hydrate(ctx context.Context, objects []model.Audit) ([]types.Audit, error) {
	items := make([]types.Audit, 0, len(objects))
	if len(objects) == 0 {
		return items, nil
	}

	userIDs := make([]int64, 0)
	clusterNames := make([]string, 0)
	datasourceIDs := make([]int64, 0)
	seenUser := map[int64]struct{}{}
	seenCluster := map[string]struct{}{}
	seenDS := map[int64]struct{}{}
	for i := range objects {
		obj := objects[i]
		if obj.OperatorId > 0 {
			if _, ok := seenUser[obj.OperatorId]; !ok {
				seenUser[obj.OperatorId] = struct{}{}
				userIDs = append(userIDs, obj.OperatorId)
			}
		}
		if obj.Cluster != "" {
			if _, ok := seenCluster[obj.Cluster]; !ok {
				seenCluster[obj.Cluster] = struct{}{}
				clusterNames = append(clusterNames, obj.Cluster)
			}
		}
		if obj.ResourceId > 0 && obj.ObjectType.IsDatasource() {
			if _, ok := seenDS[obj.ResourceId]; !ok {
				seenDS[obj.ResourceId] = struct{}{}
				datasourceIDs = append(datasourceIDs, obj.ResourceId)
			}
		}
	}

	users := map[int64]string{}
	if len(userIDs) > 0 {
		list, err := a.factory.User().List(ctx, db.WithIDIn(userIDs...), db.WithColumns("id", "name"))
		if err != nil {
			klog.Errorf("failed to get audit operator names: %v", err)
			return nil, errors.ErrServerInternal
		}
		for i := range list {
			users[list[i].Id] = list[i].Name
		}
	}
	clusters := map[string]string{}
	if len(clusterNames) > 0 {
		list, err := a.factory.Cluster().List(ctx, db.WithNameIn(clusterNames), db.WithColumns("name", "alias_name"))
		if err != nil {
			klog.Errorf("failed to get audit cluster aliases: %v", err)
			return nil, errors.ErrServerInternal
		}
		for i := range list {
			alias := list[i].AliasName
			if alias == "" {
				alias = list[i].Name
			}
			clusters[list[i].Name] = alias
		}
	}
	datasources := map[int64]string{}
	if len(datasourceIDs) > 0 {
		list, err := a.factory.Datasource().List(ctx, db.WithIDIn(datasourceIDs...), db.WithColumns("id", "name"))
		if err != nil {
			klog.Errorf("failed to get audit datasource aliases: %v", err)
			return nil, errors.ErrServerInternal
		}
		for i := range list {
			datasources[list[i].Id] = list[i].Name
		}
	}

	for i := range objects {
		item := a.model2Type(&objects[i])
		if name, ok := users[objects[i].OperatorId]; ok && name != "" {
			item.OperatorName = name
		} else {
			item.OperatorName = objects[i].Operator
		}
		if alias, ok := clusters[objects[i].Cluster]; ok {
			item.ClusterAlias = alias
		} else {
			item.ClusterAlias = objects[i].Cluster
		}
		if name, ok := datasources[objects[i].ResourceId]; ok && name != "" && objects[i].ObjectType.IsDatasource() {
			item.ResourceAlias = name
		} else if !objects[i].ObjectType.IsDatasource() || objects[i].ObjectType == model.ObjectDatasource {
			item.ResourceAlias = objects[i].ResourceName
		}
		items = append(items, *item)
	}
	return items, nil
}

func (a *audit) model2Type(o *model.Audit) *types.Audit {
	return &types.Audit{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
		Ip:                o.Ip,
		Action:            o.Action,
		Status:            o.Status,
		Operator:          o.Operator,
		Path:              o.Path,
		ObjectType:        o.ObjectType,
		Duration:          o.Duration,
		ResponseCode:      o.ResponseCode,
		Cluster:           o.Cluster,
		ResourceName:      o.ResourceName,
		ResourceNamespace: o.ResourceNamespace,
		RequestId:         o.RequestId,
		OperatorId:        o.OperatorId,
		ResourceId:        o.ResourceId,
		Operation:         o.Operation,
		ResourceKind:      o.ResourceKind,
		Detail:            o.Detail,
	}
}

func NewAudit(f db.ShareDaoFactory) *audit {
	return &audit{factory: f}
}
