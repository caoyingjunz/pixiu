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

// DefaultLogDir runner 容器日志默认根目录（runtime.log_dir 未配置时使用）。
// 默认值只此一处：config 侧不做同值兜底，空 log_dir 交由 setLogDir 的 no-op 语义落到这里。
const DefaultLogDir = "/var/lib/pixiu/runner-logs"

var (
	logDirMu sync.RWMutex
	logDir   = DefaultLogDir
)

// logPath 返回容器日志文件的确定性路径 <logDir>/<容器名>.log。
// 该路径是日志位置在全局的唯一来源：RunContainer 按它落盘、Logs 按它读取，
// 不接受调用方传入。关键在于路径只依赖「容器名 + runtime.log_dir」——只知道容器名的
// 读取方（SSE 日志接口、重启后的进程、多实例中的另一实例）因此都能定位到同一个文件。
func logPath(name string) string {
	logDirMu.RLock()
	defer logDirMu.RUnlock()
	return filepath.Join(logDir, name+".log")
}

// setLogDir 由 New 在构造运行时时写入，作为 logPath 的全局来源
func setLogDir(dir string) {
	if dir == "" {
		return
	}
	logDirMu.Lock()
	logDir = dir
	logDirMu.Unlock()
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
	// WaitTimeout 等待容器退出的最长时长，来自 worker.deploy_timeout；
	// <=0 时各实现回落到自身的默认上限
	WaitTimeout time.Duration
}

// Runtime 是宿主容器运行时的抽象。
// 注意：与本仓库 types.RuntimeSpec（被部署集群的 CRI）不是一回事，不要混用。
type Runtime interface {
	Kind() Kind
	// RunContainer 清理同名旧容器 → 创建 → 启动 → 等待退出；退出码非 0 返回错误
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
	// 日志根目录在这里落地，保证 logPath 与本次构造的配置一致
	setLogDir(opts.LogDir)
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
	defaultRuntime     Runtime
	defaultRuntimeOnce sync.RWMutex
)

// Init 进程启动时初始化全局运行时，fail-fast。
func Init(opts config.RuntimeOptions) error {
	r, err := New(opts)
	if err != nil {
		return err
	}
	defaultRuntimeOnce.Lock()
	defer defaultRuntimeOnce.Unlock()
	defaultRuntime = r
	return nil
}

// Default 返回全局运行时实例；未 Init 时返回 nil，调用方需自行判空或由 Init 保证。
func Default() Runtime {
	defaultRuntimeOnce.RLock()
	defer defaultRuntimeOnce.RUnlock()
	return defaultRuntime
}
