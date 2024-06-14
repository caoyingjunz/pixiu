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

package container

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type Container struct {
	client *client.Client
	action string
	name   string
	planId int64
	dir    string
}

func NewContainer(action string, planId int64, dir string) (*Container, error) {
	client, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}

	return &Container{
		client: client,
		action: action,
		name:   fmt.Sprintf("%s-%d", action, planId),
		planId: planId,
		dir:    dir}, nil
}

// StartAndWaitForContainer 创建，启动容器，并等待容器退出
func (c *Container) StartAndWaitForContainer(ctx context.Context, image string) error {
	// 已经存在，则先删除运行的容器
	if err := c.ClearContainer(ctx); err != nil {
		return err
	}

	config := &container.Config{
		Labels: map[string]string{
			"author":    "caoyingjunz",
			"pixiuName": c.name,
		},
		Image: image,
		Env:   []string{fmt.Sprintf("COMMAND=%s", c.action)},
	}
	hostConfig := &container.HostConfig{
		Binds: []string{fmt.Sprintf("%s/%d:/configs", c.dir, c.planId)},
	}
	netConfig := &network.NetworkingConfig{}
	resp, err := c.client.ContainerCreate(ctx, config, hostConfig, netConfig, c.name)
	if err != nil {
		return err
	}

	// 启动容器
	if err = c.client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return err
	}
	// 等待容器运行完成退出
	return c.WaitContainer(ctx, resp.ID, 180)
}

func (c *Container) Close() error {
	return c.client.Close()
}

// ClearContainer 清理已存在的老容器
func (c *Container) ClearContainer(ctx context.Context) error {
	old, err := c.GetContainer(ctx, c.name)
	if err != nil {
		// 如果不存在则直接返回
		if err == errors.ErrContainerNotFound {
			return nil
		}
		return err
	}

	containerId := old.ID
	timeout := 5 * time.Second
	if err = c.client.ContainerStop(ctx, containerId, &timeout); err != nil {
		return err
	}

	return c.client.ContainerRemove(ctx, containerId, types.ContainerRemoveOptions{Force: true})
}

func (c *Container) ListContainers(ctx context.Context) ([]types.Container, error) {
	cs, err := c.client.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return nil, err
	}

	return cs, nil
}

func (c *Container) GetContainer(ctx context.Context, containerName string) (*types.Container, error) {
	containers, err := c.ListContainers(ctx)
	if err != nil {
		return nil, err
	}

	for _, container := range containers {
		for _, name := range container.Names {
			if name == "/"+containerName {
				return &container, nil
			}
		}
	}
	return nil, errors.ErrContainerNotFound
}

// WaitContainer
// 等待容器运行退出
// 官方的客户端实现有问题，先通过探针的方式规避，后续优化
// 循环检查容器状态，直到出现异常或符合预期
func (c *Container) WaitContainer(ctx context.Context, containerId string, times int) error {
	//_, errCh := c.client.ContainerWait(ctx, resp.ID, container.WaitConditionNextExit)
	//if err = <-errCh; err != nil {
	//	fmt.Println("结束", err)
	//
	//	return err
	//}

	for i := 0; i < times; i++ {
		klog.Infof("waiting for container at %d times", i+1)
		// 先等待 5s 再执行，开始等待符合业务场景，且后续的逻辑处理不受影响
		time.Sleep(5 * time.Second)

		// 实际开始检查
		containerInfo, err := c.client.ContainerInspect(ctx, containerId)
		if err != nil {
			return err
		}
		if containerInfo.State != nil {
			// Can be one of "created", "running", "paused", "restarting", "removing", "exited", or "dead"
			state := containerInfo.State
			// 容器还在运行，等待下一次检查
			if state.Status == "running" && state.Running {
				continue
			}
			// 状态异常，直接退出
			if state.Status == "paused" || state.Status == "removing" || state.Status == "dead" {
				return fmt.Errorf("容器状态异常(%s)，退出等待", state.Status)
			}

			// 容器已经退出
			if state.Status == "exited" {
				if state.ExitCode == 0 {
					// 正常退出
					return nil
				} else {
					// 异常退出返回错误信息
					return fmt.Errorf(state.Error)
				}
			}

			// 其他状态，继续等待
		}
	}

	return fmt.Errorf("等待容器(%s)运行完成超时", containerId)
}
