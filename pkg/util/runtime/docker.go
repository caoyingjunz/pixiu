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
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/cmd/app/config"
)

type dockerRuntime struct {
	client *client.Client
}

func newDockerRuntime(opts config.RuntimeOptions) (*dockerRuntime, error) {
	clientOpts := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}
	// 配置层统一裸 socket 路径；docker SDK 的 host 需要 unix:// URL，这里补前缀。
	// 留空时不覆盖，沿用 DOCKER_HOST 环境变量/默认 socket。
	if opts.Socket != "" {
		clientOpts = append(clientOpts, client.WithHost("unix://"+opts.Socket))
	}

	cli, err := client.NewClientWithOpts(clientOpts...)
	if err != nil {
		return nil, err
	}

	return &dockerRuntime{client: cli}, nil
}

func (d *dockerRuntime) Kind() Kind {
	return KindDocker
}

func (d *dockerRuntime) Close() error {
	return d.client.Close()
}

// RunContainer 创建，启动容器，并等待容器退出
func (d *dockerRuntime) RunContainer(ctx context.Context, spec *ContainerSpec) error {
	// 已经存在，则先删除运行的容器
	if err := d.RemoveContainer(ctx, spec.Name); err != nil {
		return err
	}

	cfg := &container.Config{
		Labels: spec.Labels,
		Image:  spec.Image,
		Env:    spec.Env,
	}
	hostConfig := &container.HostConfig{
		Binds: spec.Binds,
	}
	if spec.NetworkHost {
		hostConfig.NetworkMode = network.NetworkHost
	}
	netConfig := &network.NetworkingConfig{}
	resp, err := d.client.ContainerCreate(ctx, cfg, hostConfig, netConfig, nil, spec.Name)
	if err != nil {
		return err
	}

	// 启动容器
	if err = d.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return err
	}
	// 等待容器运行完成退出。times<=0 表示不设内部上限（对应 ContainerSpec.WaitTimeout<=0），
	// 等待至容器退出或 ctx 结束；>0 时按 5s 一轮换算，至少检查一轮。
	times := 0
	if spec.WaitTimeout > 0 {
		times = int(spec.WaitTimeout / (5 * time.Second))
		if times < 1 {
			times = 1
		}
	}
	return d.waitContainer(ctx, resp.ID, times)
}

// waitContainer 等待容器运行退出
// 官方的客户端实现有问题，先通过探针的方式规避，后续优化
// 循环检查容器状态，直到出现异常或符合预期
func (d *dockerRuntime) waitContainer(ctx context.Context, containerId string, times int) error {
	if times > 0 {
		klog.Infof("等待任务(%s)执行完成(最长等待时间 %d 秒)", containerId, times*5)
	} else {
		klog.Infof("等待任务(%s)执行完成(不限制等待时长)", containerId)
	}
	for i := 0; times <= 0 || i < times; i++ {
		// 上层 ctx 取消（如 agent 收到 SIGTERM）时尽快退出等待，报文与 containerd 实现口径一致
		select {
		case <-ctx.Done():
			return fmt.Errorf("任务(%s)被取消: %v", containerId, ctx.Err())
		default:
		}

		// 先等待 5s 再执行，开始等待符合业务场景，且后续的逻辑处理不受影响
		time.Sleep(5 * time.Second)

		// 实际开始检查
		containerInfo, err := d.client.ContainerInspect(ctx, containerId)
		if err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf("任务(%s)被取消: %v", containerId, ctx.Err())
			}
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
				}
				// docker 的 State.Error 对普通非零退出通常为空字符串，为空时补退出码，
				// 与 containerd 实现的错误信息保持一致
				if state.Error != "" {
					return fmt.Errorf("%s", state.Error)
				}
				return fmt.Errorf("容器(%s)执行失败，退出码 %d", containerId, state.ExitCode)
			}

			// 其他状态，继续等待
		}
	}

	return fmt.Errorf("已等待 %d 秒，任务(%s)执行超时", times*5, containerId)
}

// RemoveContainer 清理已存在的老容器；容器不存在时返回 nil
func (d *dockerRuntime) RemoveContainer(ctx context.Context, name string) error {
	containerId, found, err := d.resolveContainerId(ctx, name)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}

	timeoutSec := 5
	if err = d.client.ContainerStop(ctx, containerId, container.StopOptions{Timeout: &timeoutSec}); err != nil {
		return err
	}

	return d.client.ContainerRemove(ctx, containerId, container.RemoveOptions{Force: true})
}

// Logs 返回容器日志文本流。
// docker 原生日志流为多路复用格式（每帧 8 字节头），这里统一解复用为纯文本，
// 使接口语义对齐 containerd 实现（直接返回日志文件流），调用方无需再关心帧头。
func (d *dockerRuntime) Logs(ctx context.Context, name string, follow bool) (io.ReadCloser, error) {
	containerId, found, err := d.resolveContainerId(ctx, name)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("容器(%s)不存在", name)
	}

	ctx, cancel := context.WithCancel(ctx)
	raw, err := d.client.ContainerLogs(ctx, containerId, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Timestamps: false,
	})
	if err != nil {
		cancel()
		return nil, err
	}

	// 解复用在独立 goroutine 中完成，调用方按纯文本读取管道
	pr, pw := io.Pipe()
	go func() {
		_, copyErr := stdcopy.StdCopy(pw, pw, raw)
		_ = raw.Close()
		_ = pw.CloseWithError(copyErr)
	}()

	return &logsReadCloser{ReadCloser: pr, cancel: cancel}, nil
}

// logsReadCloser 关闭读取管道的同时取消上游请求，避免 follow 场景下 goroutine 滞留
type logsReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (l *logsReadCloser) Close() error {
	l.cancel()
	return l.ReadCloser.Close()
}

func (d *dockerRuntime) ImageExists(ctx context.Context, ref string) (bool, error) {
	if _, _, err := d.client.ImageInspectWithRaw(ctx, ref); err != nil {
		if client.IsErrNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (d *dockerRuntime) PullImage(ctx context.Context, ref string) error {
	reader, err := d.client.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()

	_, err = io.Copy(io.Discard, reader)
	return err
}

func (d *dockerRuntime) RemoveImage(ctx context.Context, ref string) error {
	_, err := d.client.ImageRemove(ctx, ref, image.RemoveOptions{Force: true})
	return err
}

// resolveContainerId 按名解析容器 ID（Inspect 支持容器名），避免全量列举容器
func (d *dockerRuntime) resolveContainerId(ctx context.Context, name string) (string, bool, error) {
	info, err := d.client.ContainerInspect(ctx, name)
	if err != nil {
		if client.IsErrNotFound(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return info.ID, true, nil
}

var _ Runtime = (*dockerRuntime)(nil)
