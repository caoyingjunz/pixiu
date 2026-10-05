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
	"path/filepath"
	"sync"
	"time"

	"github.com/caoyingjunz/pixiu/cmd/app/config"
)

type Kind string

const (
	KindDocker     Kind = "docker"
	KindContainerd Kind = "containerd"
)

// DefaultLogDir runner 容器日志根目录，固定不可配置；容器日志固定落 <DefaultLogDir>/<容器名>.log。
// 目录位于 /etc/pixiu 卷内：随 pixiu 容器的 /etc/pixiu 挂载天然持久化，无需独立日志卷。
const DefaultLogDir = "/etc/pixiu/runner-logs"

// logPath 返回容器日志文件的确定性路径 <DefaultLogDir>/<容器名>.log。
// 该路径是日志位置在全局的唯一来源：RunContainer 按它落盘、Logs 按它读取，
// 不接受调用方传入。关键在于路径只由容器名决定，根目录固定 DefaultLogDir——只知道
// 容器名的读取方（SSE 日志接口、重启后的进程、多实例中的另一实例）因此都能定位到同一个文件。
func logPath(name string) string {
	return filepath.Join(DefaultLogDir, name+".log")
}

// ContainerSpec 描述一个一次性任务容器的期望状态。
// 刻意抽掉 docker/containerd 的差异，只保留 runner 容器真正需要的字段。
type ContainerSpec struct {
	Name        string // 容器名，约定 <action>-<planId>，如 deploy-12
	Image       string
	Env         []string // KEY=VALUE
	Binds       []string // host:container，如 /etc/pixiu/plan/12:/configs
	NetworkHost bool     // 是否使用宿主网络
	Labels      map[string]string
	// WaitTimeout 等待容器退出的最长时长；<=0 表示不设内部上限（等待容器退出或 ctx 结束）
	WaitTimeout time.Duration
}

// Runtime 是宿主容器运行时的抽象。
// 注意：与本仓库 types.RuntimeSpec（被部署集群的 CRI）不是一回事，不要混用。
type Runtime interface {
	Kind() Kind
	// RunContainer 清理同名旧容器 → 创建 → 启动 → 等待退出；退出码非 0 返回错误。
	// 执行结束后容器与任务保留（对齐 docker 语义），同名容器在下一次执行开始时由实现统一清理。
	RunContainer(ctx context.Context, spec *ContainerSpec) error
	// RemoveContainer 停止并删除同名容器；容器不存在时返回 nil
	RemoveContainer(ctx context.Context, name string) error
	// Logs 返回容器日志读取流；follow 为 true 时持续跟随
	Logs(ctx context.Context, name string, follow bool) (io.ReadCloser, error)
	// 镜像操作
	ImageExists(ctx context.Context, ref string) (bool, error)
	PullImage(ctx context.Context, ref string) error
	RemoveImage(ctx context.Context, ref string) error
	Close() error
}

// New 按配置构造运行时实例。
func New(opts config.RuntimeOptions) (Runtime, error) {
	switch Kind(opts.CRI) {
	case KindDocker:
		return newDockerRuntime(opts)
	case KindContainerd:
		return newContainerdRuntime(opts)
	default:
		return nil, fmt.Errorf("未知的容器运行时类型(%s)，可选值: docker, containerd", opts.CRI)
	}
}

var (
	defaultRuntime   Runtime
	defaultRuntimeMu sync.Mutex
	// 懒加载：Init 仅登记配置，Default 首次实际使用时才建立连接。
	// 连接成功后缓存到 defaultRuntime；失败不缓存——后续调用会重试，
	// containerd 后启动的场景无需重启 pixiu。
	defaultOptions config.RuntimeOptions
	inited         bool
)

// Init 登记宿主容器运行时配置，不做任何连接（懒加载）。
// 配置合法性（CRI 取值、socket 路径）由启动链路 Config.Valid 校验，此处不再 fail-fast；
// 连接失败只在实际使用（Default）时报错，不阻断 pixiu 启动。
func Init(opts config.RuntimeOptions) {
	defaultRuntimeMu.Lock()
	defer defaultRuntimeMu.Unlock()
	defaultOptions = opts
	inited = true
}

// Default 返回全局运行时实例；首次调用时按 Init 登记的配置建立连接（懒加载）。
// 未初始化或连接失败时返回错误；失败不缓存，后续调用会重试。
func Default() (Runtime, error) {
	defaultRuntimeMu.Lock()
	defer defaultRuntimeMu.Unlock()

	if defaultRuntime != nil {
		return defaultRuntime, nil
	}
	if !inited {
		return nil, fmt.Errorf("宿主容器运行时未初始化")
	}

	r, err := New(defaultOptions)
	if err != nil {
		return nil, err
	}
	defaultRuntime = r
	return r, nil
}
