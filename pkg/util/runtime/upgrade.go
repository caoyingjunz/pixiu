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

package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/moby/moby/client"
)

// ContainerImage 返回容器当前使用的镜像引用。
func (d *dockerRuntime) ContainerImage(ctx context.Context, name string) (string, error) {
	result, err := d.client.ContainerInspect(ctx, name, client.ContainerInspectOptions{})
	if err != nil {
		return "", err
	}
	info := result.Container
	if info.Config == nil || info.Config.Image == "" {
		return "", fmt.Errorf("容器(%s)缺少镜像信息", name)
	}
	return info.Config.Image, nil
}

// ReplaceContainer 按原容器配置用 newImage 重建容器，返回旧镜像引用。
// 镜像拉取在删除旧容器之前完成，避免拉取失败导致服务中断。
func (d *dockerRuntime) ReplaceContainer(ctx context.Context, name, newImage string) (string, error) {
	result, err := d.client.ContainerInspect(ctx, name, client.ContainerInspectOptions{})
	if err != nil {
		return "", err
	}
	info := result.Container
	if info.Config == nil {
		return "", fmt.Errorf("容器(%s)缺少配置信息", name)
	}
	oldImage := info.Config.Image

	cfg := *info.Config
	cfg.Image = newImage
	cfg.Hostname = "" // 由引擎重新分配

	if err = d.PullImage(ctx, newImage); err != nil {
		return oldImage, err
	}
	if err = d.RemoveContainer(ctx, name); err != nil {
		return oldImage, err
	}
	resp, err := d.client.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config:     &cfg,
		HostConfig: info.HostConfig,
		Name:       name,
	})
	if err != nil {
		return oldImage, err
	}
	if _, err = d.client.ContainerStart(ctx, resp.ID, client.ContainerStartOptions{}); err != nil {
		return oldImage, err
	}
	return oldImage, nil
}

// nerdctlContainerInfo 是 nerdctl inspect 输出的子集（与 docker inspect 字段对齐）。
type nerdctlContainerInfo struct {
	Name   string `json:"Name"`
	Image  string `json:"Image"`
	Config struct {
		Image      string   `json:"Image"`
		Env        []string `json:"Env"`
		Cmd        []string `json:"Cmd"`
		Entrypoint []string `json:"Entrypoint"`
	} `json:"Config"`
	HostConfig struct {
		NetworkMode   string `json:"NetworkMode"`
		Privileged    bool   `json:"Privileged"`
		RestartPolicy struct {
			Name string `json:"Name"`
		} `json:"RestartPolicy"`
		Binds []string `json:"Binds"`
	} `json:"HostConfig"`
	Mounts []struct {
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
		RW          bool   `json:"RW"`
	} `json:"Mounts"`
}

// ContainerImage 返回容器当前使用的镜像引用。
func (c *containerdRuntime) ContainerImage(ctx context.Context, name string) (string, error) {
	infos, err := c.nerdctlInspect(ctx, name)
	if err != nil {
		return "", err
	}
	if len(infos) == 0 {
		return "", fmt.Errorf("容器(%s)不存在", name)
	}
	image := infos[0].Config.Image
	if image == "" {
		image = infos[0].Image
	}
	if image == "" {
		return "", fmt.Errorf("容器(%s)缺少镜像信息", name)
	}
	return image, nil
}

// ReplaceContainer 借助 nerdctl 按原容器配置用 newImage 重建容器，返回旧镜像引用。
// containerd 本身没有 restart 策略，rebuild 走 nerdctl run 以保留 --restart 等启动语义。
func (c *containerdRuntime) ReplaceContainer(ctx context.Context, name, newImage string) (string, error) {
	infos, err := c.nerdctlInspect(ctx, name)
	if err != nil {
		return "", err
	}
	if len(infos) == 0 {
		return "", fmt.Errorf("容器(%s)不存在", name)
	}
	info := infos[0]
	oldImage := info.Config.Image
	if oldImage == "" {
		oldImage = info.Image
	}

	// 先拉镜像，再动旧容器，避免拉取失败导致服务中断
	if err = c.nerdctlExec(ctx, "pull", newImage); err != nil {
		return oldImage, err
	}
	if err = c.nerdctlExec(ctx, "rm", "-f", name); err != nil {
		return oldImage, err
	}

	args := []string{"run", "-d", "--name", name}
	if policy := info.HostConfig.RestartPolicy.Name; policy != "" && policy != "no" {
		args = append(args, "--restart", policy)
	}
	if networkMode := info.HostConfig.NetworkMode; networkMode != "" && networkMode != "default" {
		args = append(args, "--net", networkMode)
	}
	if info.HostConfig.Privileged {
		args = append(args, "--privileged=true")
	}

	seen := make(map[string]struct{})
	if len(info.Mounts) > 0 {
		for _, mount := range info.Mounts {
			if mount.Source == "" || mount.Destination == "" {
				continue
			}
			if _, ok := seen[mount.Destination]; ok {
				continue
			}
			seen[mount.Destination] = struct{}{}
			bind := mount.Source + ":" + mount.Destination
			if !mount.RW {
				bind += ":ro"
			}
			args = append(args, "-v", bind)
		}
	} else {
		for _, bind := range info.HostConfig.Binds {
			args = append(args, "-v", bind)
		}
	}

	for _, env := range info.Config.Env {
		args = append(args, "-e", env)
	}
	if len(info.Config.Entrypoint) > 1 {
		return oldImage, fmt.Errorf("容器(%s) entrypoint 包含多段参数，暂不支持自动重建", name)
	}
	if len(info.Config.Entrypoint) == 1 {
		args = append(args, "--entrypoint", info.Config.Entrypoint[0])
	}

	args = append(args, newImage)
	args = append(args, info.Config.Cmd...)
	if err = c.nerdctlExec(ctx, args...); err != nil {
		return oldImage, err
	}
	return oldImage, nil
}

func (c *containerdRuntime) nerdctlInspect(ctx context.Context, name string) ([]nerdctlContainerInfo, error) {
	out, err := c.nerdctlOutput(ctx, "inspect", name)
	if err != nil {
		return nil, err
	}
	var infos []nerdctlContainerInfo
	if err = json.Unmarshal(out, &infos); err != nil {
		return nil, fmt.Errorf("解析 nerdctl inspect 输出失败: %v", err)
	}
	return infos, nil
}

func (c *containerdRuntime) nerdctlOutput(ctx context.Context, args ...string) ([]byte, error) {
	full := append([]string{"-n", c.namespace}, args...)
	cmd := exec.CommandContext(ctx, c.nerdctl, full...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("nerdctl %s 失败: %v: %s", strings.Join(full, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func (c *containerdRuntime) nerdctlExec(ctx context.Context, args ...string) error {
	_, err := c.nerdctlOutput(ctx, args...)
	return err
}
