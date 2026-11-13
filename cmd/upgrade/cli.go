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

package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/cmd/app/config"
)

// containerCLI 通过宿主机的 nerdctl/docker 命令行管理 pixiu 容器，
// 与手动 update.sh 的 pull → stop → rm → run 流程保持一致。
// 由 pixiu-upgrade 进程实现 upgrader.ContainerCLI 接口。
type containerCLI struct {
	bin  string   // nerdctl / docker
	args []string // nerdctl 时含 ["-n","default"]，docker 为空
}

// newContainerCLI 按运行时配置选择命令行工具：containerd 使用 nerdctl，
// docker 使用 docker。CRI 合法性由配置校验保证。
func newContainerCLI(rt config.RuntimeOptions) *containerCLI {
	if rt.CRI == "docker" {
		return &containerCLI{bin: "docker"}
	}
	return &containerCLI{bin: "nerdctl", args: []string{"-n", "default"}}
}

// command 拼装完整命令（bin + 固定参数 + 业务参数）。
func (c *containerCLI) command(ctx context.Context, args ...string) *exec.Cmd {
	full := make([]string, 0, len(c.args)+len(args))
	full = append(full, c.args...)
	full = append(full, args...)
	return exec.CommandContext(ctx, c.bin, full...)
}

// exec 执行命令并合并 stdout/stderr；失败时错误包含命令与输出。
func (c *containerCLI) exec(ctx context.Context, args ...string) (string, error) {
	cmd := c.command(ctx, args...)
	commandLine := strings.Join(cmd.Args, " ")
	klog.Infof("[升级] 执行命令: %s", commandLine)

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	out := strings.TrimSpace(output.String())
	if err != nil {
		return out, fmt.Errorf("执行命令(%s)失败: %v，输出: %s", commandLine, err, out)
	}
	return out, nil
}

// Pull 拉取镜像。
func (c *containerCLI) Pull(ctx context.Context, image string) error {
	_, err := c.exec(ctx, "pull", image)
	return err
}

// Stop 停止容器；容器不存在时视为成功。
func (c *containerCLI) Stop(ctx context.Context, name string) error {
	_, err := c.exec(ctx, "stop", name)
	if err != nil && isNotFoundOutput(err.Error()) {
		return nil
	}
	return err
}

// Remove 删除容器；容器不存在时视为成功。
// 调用方须先执行 Stop（与手动更新脚本顺序一致）。
func (c *containerCLI) Remove(ctx context.Context, name string) error {
	_, err := c.exec(ctx, "rm", name)
	if err != nil && isNotFoundOutput(err.Error()) {
		return nil
	}
	return err
}

// Run 以 runArgs 启动新容器。
// runArgs 按空格拆分，不支持含空格的引号参数（需与手动 run 参数保持一致，不含 --name 与镜像）。
func (c *containerCLI) Run(ctx context.Context, name, image, runArgs string) error {
	args := []string{"run", "-d"}
	args = append(args, strings.Fields(runArgs)...)
	args = append(args, "--name", name, image)
	_, err := c.exec(ctx, args...)
	return err
}

// IsRunning 返回容器是否处于运行状态；容器不存在时返回 false, nil。
func (c *containerCLI) IsRunning(ctx context.Context, name string) (bool, error) {
	out, err := c.exec(ctx, "inspect", "--format", "{{.State.Running}}", name)
	if err != nil {
		if isNotFoundOutput(err.Error()) {
			return false, nil
		}
		return false, err
	}
	return strings.TrimSpace(out) == "true", nil
}

// isNotFoundOutput 判断命令输出是否表示容器/对象不存在（忽略大小写）。
func isNotFoundOutput(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "no such container") || strings.Contains(lower, "no such object") || strings.Contains(lower, "not found")
}
