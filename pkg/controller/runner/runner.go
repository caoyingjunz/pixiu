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

package runner

import (
	"context"
	"fmt"
	"io"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type RunnerGetter interface {
	Runner() Interface
}

type Interface interface {
	Create(ctx context.Context, req *types.CreateRunnerRequest) error
	Update(ctx context.Context, req *types.UpdateRunnerRequest) error
	Delete(ctx context.Context, runnerId int64) error
	Get(ctx context.Context, runnerId int64) (*types.Runner, error)
	List(ctx context.Context, listOption types.ListOptions) (interface{}, error)

	Install(ctx context.Context, req types.InstallRunnerRequest) error
	UnInstall(ctx context.Context, req types.UninstallRunnerRequest) error
}

type runnerController struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func NewRunner(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &runnerController{
		cc:      cfg,
		factory: f,
	}
}

func (r *runnerController) Create(ctx context.Context, req *types.CreateRunnerRequest) error {
	object := &model.Runner{
		Name:        req.Name,
		EngineImage: req.EngineImage,
		Status:      req.Status,
		Description: req.Description,
	}

	if _, err := r.factory.Runner().Create(ctx, object); err != nil {
		klog.Errorf("failed to create runner %s: %v", req.Name, err)
		return errors.ErrServerInternal
	}
	return nil
}

func (r *runnerController) Update(ctx context.Context, req *types.UpdateRunnerRequest) error {
	runnerId := req.Id

	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.EngineImage != nil {
		updates["engine_image"] = *req.EngineImage
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}

	if err := r.factory.Runner().Update(ctx, runnerId, *req.ResourceVersion, updates); err != nil {
		klog.Errorf("failed to update runner %d: %v", runnerId, err)
		return errors.ErrServerInternal
	}
	return nil
}

func (r *runnerController) Delete(ctx context.Context, runnerId int64) error {
	if _, err := r.factory.Runner().Delete(ctx, runnerId); err != nil {
		klog.Errorf("failed to delete runner %d: %v", runnerId, err)
		return errors.ErrServerInternal
	}
	return nil
}

func (r *runnerController) Get(ctx context.Context, runnerId int64) (*types.Runner, error) {
	object, err := r.factory.Runner().Get(ctx, runnerId)
	if err != nil {
		klog.Errorf("failed to get runner %d: %v", runnerId, err)
		return nil, errors.ErrServerInternal
	}
	if object == nil {
		return nil, errors.ErrRunnerNotFound
	}
	return model2Type(object), nil
}

func (r *runnerController) List(ctx context.Context, listOption types.ListOptions) (interface{}, error) {
	listOption.SetDefaultPageOption()

	pageResult := types.PageResult{
		PageRequest: types.PageRequest{
			Page:  listOption.Page,
			Limit: listOption.Limit,
		},
	}

	opts := []db.Options{
		db.WithNameLike(listOption.NameSelector),
	}

	var err error
	pageResult.Total, err = r.factory.Runner().Count(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to get runners count: %v", err)
		pageResult.Message = err.Error()
		return nil, err
	}

	offset := (listOption.Page - 1) * listOption.Limit
	opts = append(opts, []db.Options{
		db.WithModifyOrderByDesc(),
		db.WithOffset(offset),
		db.WithLimit(listOption.Limit),
	}...)

	objects, err := r.factory.Runner().List(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to list runners: %v", err)
		pageResult.Message = err.Error()
		return nil, errors.ErrServerInternal
	}

	var ts []types.Runner
	for _, object := range objects {
		ts = append(ts, *model2Type(&object))
	}
	pageResult.Items = ts
	return pageResult, nil
}

func (r *runnerController) Install(ctx context.Context, req types.InstallRunnerRequest) error {
	// 1. 查询现有 runner
	obj, err := r.factory.Runner().Get(ctx, req.Id)
	if err != nil || obj == nil {
		klog.Errorf("failed to get runner %d: %v", req.Id, err)
		return errors.ErrServerInternal
	}

	// 2. 更新 runner 状态为正在安装
	if err = r.updateStatus(ctx, req.Id, model.RunnerStatusInstalling); err != nil {
		return err
	}

	// 启动异步任务，使用独立的 context
	go func(runnerId int64, image string) {
		// 创建独立的 context，不随主请求结束而结束
		pullCtx := context.Background()
		status := model.RunnerStatusInstalled

		// 3. 调用 docker pull 拉取镜像
		if pullErr := r.pullImage(pullCtx, image); pullErr != nil {
			status = model.RunnerStatusUnknown
			klog.Errorf("failed to pull image %s for runner %d: %v", image, runnerId, pullErr)
		}
		// 更新最终状态
		if updateErr := r.updateStatus(pullCtx, runnerId, status); updateErr != nil {
			klog.Errorf("failed to update runner %d status to %d: %v", runnerId, status, updateErr)
		}
	}(req.Id, obj.EngineImage)

	return nil
}

func (r *runnerController) UnInstall(ctx context.Context, req types.UninstallRunnerRequest) error {
	// 1. 查询现有 runner
	obj, err := r.factory.Runner().Get(ctx, req.Id)
	if err != nil || obj == nil {
		klog.Errorf("failed to get runner %d: %v", req.Id, err)
		return errors.ErrServerInternal
	}

	// 2. 更新 runner 状态为卸载中
	if err = r.updateStatus(ctx, req.Id, model.RunnerStatusUnInstalling); err != nil {
		return err
	}

	// 启动异步任务，使用独立的 context
	go func(runnerId int64, image string) {
		removeCtx := context.Background()
		status := model.RunnerStatusUnstart

		// 3. 调用 docker remove 移除镜像，忽略报错
		if pullErr := r.removeImage(removeCtx, image); pullErr != nil {
			klog.Errorf("failed to remove image %s for runner %d: %v", image, runnerId, pullErr)
		}
		if updateErr := r.updateStatus(removeCtx, runnerId, status); updateErr != nil {
			klog.Errorf("failed to update runner %d status to %d: %v", runnerId, status, updateErr)
		}
	}(req.Id, obj.EngineImage)

	return nil
}

// 更新 runner 状态
func (r *runnerController) updateStatus(ctx context.Context, runnerId int64, status model.RunnerStatus) error {
	return r.factory.Runner().InternalUpdate(ctx, runnerId, map[string]interface{}{"status": status})
}

// 拉取 docker 镜像
func (r *runnerController) pullImage(ctx context.Context, imageName string) error {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		klog.Errorf("failed to create docker client for pull %s: %v", imageName, err)
		return fmt.Errorf("%s: %w", imageName, err)
	}
	defer cli.Close()

	reader, err := cli.ImagePull(ctx, imageName, dockertypes.ImagePullOptions{})
	if err != nil {
		klog.Errorf("failed to pull image %s: %v", imageName, err)
		return fmt.Errorf("%s: %w", imageName, err)
	}
	defer reader.Close()

	if _, err = io.Copy(io.Discard, reader); err != nil {
		klog.Errorf("failed to read pull output for image %s: %v", imageName, err)
		return fmt.Errorf("%s: %w", imageName, err)
	}

	klog.Infof("successfully pulled image %s", imageName)
	return nil
}

// 移除 docker 镜像
func (r *runnerController) removeImage(ctx context.Context, imageName string) error {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		klog.Errorf("failed to create docker client for remove %s: %v", imageName, err)
		return fmt.Errorf("%s: %w", imageName, err)
	}
	defer cli.Close()

	if _, err = cli.ImageRemove(ctx, imageName, dockertypes.ImageRemoveOptions{Force: true}); err != nil {
		klog.Errorf("failed to remove image %s: %v", imageName, err)
		return fmt.Errorf("%s: %w", imageName, err)
	}

	klog.Infof("successfully removed image %s", imageName)
	return nil
}

func model2Type(o *model.Runner) *types.Runner {
	return &types.Runner{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
		Name:        o.Name,
		EngineImage: o.EngineImage,
		Status:      o.Status,
		Description: o.Description,
	}
}
