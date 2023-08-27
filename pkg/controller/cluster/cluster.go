/*
Copyright 2021 The Pixiu Authors.

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

package cluster

import (
	"context"
	"fmt"

	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type ClusterGetter interface {
	Cluster() Interface
}

type Interface interface {
	Create(ctx context.Context, clu *types.Cluster) error
	Update(ctx context.Context, cid int64, clu *types.Cluster) error
	Delete(ctx context.Context, cid int64) error
	Get(ctx context.Context, cid int64) (*types.Cluster, error)
	List(ctx context.Context) ([]types.Cluster, error)
}

type cluster struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func (c *cluster) preCreate(ctx context.Context, clu *types.Cluster) error {
	// TODO: 集群名称必须是由英文，数字组成
	if len(clu.Name) == 0 {
		return fmt.Errorf("创建 kubernetes 集群时，集群名称不允许为空")
	}

	if len(clu.KubeConfig) == 0 {
		return fmt.Errorf("创建 kubernetes 集群时， kubeconfig 不允许为空")
	}

	// TODO: 创建前确保连通性
	return nil
}

func (c *cluster) Create(ctx context.Context, clu *types.Cluster) error {
	if err := c.preCreate(ctx, clu); err != nil {
		return err
	}

	// 执行创建
	_, err := c.factory.Cluster().Create(ctx, &model.Cluster{
		Name:        clu.Name,
		AliasName:   clu.AliasName,
		KubeConfig:  clu.KubeConfig,
		Description: clu.Description,
	})
	if err != nil {
		return err
	}

	// TODO: 暂时不做创建后动作
	return nil
}

// TODO:
func (c *cluster) postCreate(ctx context.Context, cid int64, clu *types.Cluster) error {
	return nil
}

func (c *cluster) Update(ctx context.Context, cid int64, clu *types.Cluster) error {
	return nil
}

// 删除前置检查
func (c *cluster) preDelete(ctx context.Context, cid int64) error {
	// TODO
	return nil
}

func (c *cluster) Delete(ctx context.Context, cid int64) error {
	if err := c.preDelete(ctx, cid); err != nil {
		return err
	}
	// TODO: 其他场景补充
	return c.factory.Cluster().Delete(ctx, cid)
}

func (c *cluster) Get(ctx context.Context, cid int64) (*types.Cluster, error) {
	object, err := c.factory.Cluster().Get(ctx, cid)
	if err != nil {
		return nil, err
	}

	return model2Type(object), nil
}

func model2Type(o *model.Cluster) *types.Cluster {
	return &types.Cluster{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
		Name:        o.Name,
		AliasName:   o.AliasName,
		Description: o.Description,
	}
}

func (c *cluster) List(ctx context.Context) ([]types.Cluster, error) {
	objects, err := c.factory.Cluster().List(ctx)
	if err != nil {
		return nil, err
	}

	var cs []types.Cluster
	for _, object := range objects {
		cs = append(cs, *model2Type(&object))
	}

	return cs, nil
}

func NewCluster(cfg config.Config, f db.ShareDaoFactory) *cluster {
	return &cluster{
		cc:      cfg,
		factory: f,
	}
}
