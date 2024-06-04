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

	"github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type Container struct {
	client *client.Client
	action string
	name   string
}

func NewContainer(action string, name string) (*Container, error) {
	client, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}

	return &Container{client: client, action: action, name: name}, nil
}

// StartAndWaitForContainer 创建，启动容器，并等待容器退出
func (c *Container) StartAndWaitForContainer(ctx context.Context) error {
	// 已经存在，则先删除运行的容器
	if err := c.ClearContainer(ctx); err != nil {
		return err
	}

	config := &container.Config{
		Labels: map[string]string{
			"author":    "caoyingjunz",
			"pixiuName": c.name,
		},
		Image: "jacky06/kubez-ansible:v3.0.1",
		Env:   []string{fmt.Sprintf("ACTION=%s", c.action)},
	}
	hostConfig := &container.HostConfig{}
	netConfig := &network.NetworkingConfig{}
	resp, err := c.client.ContainerCreate(ctx, config, hostConfig, netConfig, c.name)
	if err != nil {
		return err
	}

	if err = c.client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return err
	}

	// 等待容器运行退出
	//_, errCh := c.client.ContainerWait(ctx, resp.ID, container.WaitConditionNextExit)
	//if err = <-errCh; err != nil {
	//	fmt.Println("结束", err)
	//
	//	return err
	//}

	return nil
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
