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

package plan

import (
	"context"
	"fmt"
	"io"

	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"k8s.io/klog/v2"
)

// Runner 部署前确保本地已存在 runner 镜像
type Runner struct {
	handlerTask

	image   string
	factory db.ShareDaoFactory
}

func (r Runner) Name() string      { return "前置准备" }
func (r Runner) GetAction() string { return "runner" }

func (r Runner) Run() error {
	imageName := r.image
	ctx := context.Background()

	// 更新 runner 的数据库状态
	obj, runnerErr := r.factory.Runner().GetBy(ctx, db.WithEngineImage(imageName))
	if runnerErr == nil {
		// 如果已安装，则返回
		if obj.Status == model.RunnerStatusInstalled {
			klog.Infof("Runner %s is already installed", imageName)
			return nil
		}
		// 否则的话，进行安装
		klog.Infof("Runner %s is not installed, installing", imageName)
		_ = r.updateStatus(ctx, obj.Id, model.RunnerStatusInstalling)
	} else {
		klog.Warningf("get runner by %s: %v", imageName, runnerErr)
	}

	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return fmt.Errorf("create docker client: %w", err)
	}
	defer cli.Close()

	if _, _, err = cli.ImageInspectWithRaw(ctx, imageName); err == nil {
		klog.Infof("runner image %s already exists locally, skip pull", imageName)
		return nil
	} else if !client.IsErrNotFound(err) {
		return fmt.Errorf("inspect image %s: %w", imageName, err)
	}

	klog.Infof("runner image %s not found locally, pulling", imageName)
	reader, err := cli.ImagePull(ctx, imageName, dockertypes.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("pull image %s: %w", imageName, err)
	}
	defer reader.Close()

	if _, err = io.Copy(io.Discard, reader); err != nil {
		return fmt.Errorf("read pull output for image %s: %w", imageName, err)
	}

	klog.Infof("successfully pulled runner image %s", imageName)
	if runnerErr == nil {
		_ = r.updateStatus(ctx, obj.Id, model.RunnerStatusInstalled)
	}
	return nil
}

func (r Runner) updateStatus(ctx context.Context, runnerId int64, status model.RunnerStatus) error {
	return r.factory.Runner().InternalUpdate(ctx, runnerId, map[string]interface{}{"status": status})
}
