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

// Package upgrade 实现 pixiu 的自动升级（仅支持本地部署模式）。
//
// 整体流程（见 docs/auto-upgrade.md）：
//  1. 守护 job 定时从外部版本号地址（version_url）获取最新版本；
//  2. 与当前运行版本比较，有新增时先拉取新镜像；
//  3. 把重建规格（原容器的环境变量/挂载/网络/镜像等）写入状态文件（位于 /etc/pixiu 挂载卷内，
//     升级过程中不丢失），随后以新镜像拉起一个一次性的「升级执行器」容器；
//  4. 执行器作为独立容器任务，等主进程就绪后删除旧 pixiu 容器、用新镜像重建服务容器并写回状态。
//
// 为什么需要执行器：pixiu 自身容器被删除时，容器内的进程（包括发起重建的守护 job）也会被终止，
// 自我重建存在「先停自己、谁来起新」的矛盾；执行器是宿主 CRI 上的独立任务，不受旧容器生命周期影响。
package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/util/runtime"
)

const (
	// DefaultStateDir 升级状态目录。必须位于 /etc/pixiu 挂载卷内：
	// 主容器与执行器容器通过同一宿主目录共享状态，且宿主重启后状态不丢失
	DefaultStateDir = "/etc/pixiu/upgrade"

	stateFileName = "state.json"

	// DefaultContainerName pixiu 服务容器名
	DefaultContainerName = "pixiu"
	// DefaultExecutable pixiu 二进制在镜像内的路径，用于以新镜像拉起升级执行器
	DefaultExecutable = "/app"
	// DefaultSchedule 默认检查周期（cron 表达式）：每 30 分钟
	DefaultSchedule = "*/30 * * * *"

	// executorStartupDelay 执行器启动后等待时长：给守护 job 留出写日志、返回的时间，
	// 也避免状态文件尚未刷盘就被读取
	executorStartupDelay = 5 * time.Second
)

// State 升级状态，持久化在 <StateDir>/state.json。
type State struct {
	// Version 当前已应用的版本号（首次运行采纳基线时写入，每次升级成功后更新）
	Version string `json:"version"`
	// Image 当前已应用的完整镜像引用
	Image string `json:"image"`
	// ContainerName pixiu 服务容器名
	ContainerName string `json:"container_name"`
	// UpdatedAt 最近一次状态写入时间
	UpdatedAt time.Time `json:"updated_at"`
	// Pending 非空表示一次升级已启动、尚未确认完成；执行器成功重建后清空
	Pending *PendingUpgrade `json:"pending,omitempty"`
}

// PendingUpgrade 一次待应用（进行中）的升级。
type PendingUpgrade struct {
	TargetVersion string `json:"target_version"`
	TargetImage   string `json:"target_image"`
	// OldImage 升级前容器运行的镜像，重建失败时回滚用
	OldImage string `json:"old_image"`
	// Spec 新服务容器的重建规格（Image 字段即 TargetImage）
	Spec *runtime.ServiceSpec `json:"spec"`
	// RuntimeOpts 执行器独立连接宿主 CRI 所需的运行时配置（自主进程配置中快照）
	RuntimeOpts config.RuntimeOptions `json:"runtime_opts"`
	// Error 执行器侧最近一次失败原因（成功重建后整个 Pending 被清空）
	Error string `json:"error,omitempty"`
}

// Upgrader 自动升级守护器。以 jobmanager job 的形式周期运行（见 job.go）。
type Upgrader struct {
	cfg config.UpgradeOptions
	// rtOpts 主进程配置中的宿主 CRI 连接参数，随待升级状态快照给执行器使用
	rtOpts config.RuntimeOptions

	stateDir string

	// rt 运行时懒加载：首次升级检查时才连接宿主 CRI，连接失败不缓存（下次重试）
	rt     runtime.Runtime
	rtErr  error
	rtOnce sync.Once

	client *httpTimeoutClient
}

// New 构造自动升级守护器。cfg.Enabled 为 false 时不会被注册调度（见 options.Complete）。
func New(cfg config.UpgradeOptions, rtOpts config.RuntimeOptions) *Upgrader {
	return &Upgrader{
		cfg:    cfg,
		rtOpts: rtOpts,
		client: newHTTPTimeoutClient(),
	}
}

func (u *Upgrader) statePath() string {
	return filepath.Join(u.stateDir, stateFileName)
}

// runtime 懒加载宿主运行时。
func (u *Upgrader) runtime() (runtime.Runtime, error) {
	u.rtOnce.Do(func() {
		u.rt, u.rtErr = runtime.New(u.rtOpts)
	})
	return u.rt, u.rtErr
}

// Upgrade 执行一轮升级检查；返回值只表达本轮检查/升级动作是否出错。
// 没有新版本、目标版本更旧等属正常结果，返回 nil。
func (u *Upgrader) Upgrade(ctx context.Context) error {
	rt, err := u.runtime()
	if err != nil {
		return fmt.Errorf("连接宿主容器运行时失败: %v", err)
	}

	info, err := rt.InspectContainer(ctx, u.cfg.ContainerName)
	if err != nil {
		if err == runtime.ErrContainerNotFound {
			// 未以本地容器方式运行（如 k8s/helm 部署）：自动升级不支持，静默跳过并提示一次
			klog.Warningf("未发现本地部署容器(%s)，自动升级仅支持本地部署模式，本轮跳过", u.cfg.ContainerName)
			return nil
		}
		return fmt.Errorf("检查 pixiu 容器(%s)失败: %v", u.cfg.ContainerName, err)
	}

	latest, err := fetchExternalVersion(ctx, u.client, u.cfg.VersionURL)
	if err != nil {
		return fmt.Errorf("获取外部版本号失败(%s): %v", u.cfg.VersionURL, err)
	}
	if latest == "" {
		klog.Warningf("外部版本号地址(%s)返回空版本，本轮跳过", u.cfg.VersionURL)
		return nil
	}

	state := u.loadState()
	current, err := u.resolveCurrentVersion(state, info)
	if err != nil {
		// 首次运行且无法从镜像 tag 解析出版本（如 :latest）：采纳外部版本为基线，只记录不升级，
		// 避免每次都对未知现状执行重建
		klog.Infof("无法确定当前运行版本(%v)，采纳外部版本(%s)作为基线", err, latest)
		state.Version = latest
		state.Image = info.Image
		state.ContainerName = u.cfg.ContainerName
		state.UpdatedAt = time.Now()
		return u.saveState(state)
	}

	switch CompareVersions(latest, current) {
	case 0:
		return nil
	case -1:
		klog.Warningf("外部版本(%s)不高于当前版本(%s)，本轮跳过", latest, current)
		return nil
	}

	klog.Infof("发现新版本: %s -> %s，开始自动升级", current, latest)
	return u.doUpgrade(ctx, rt, state, info, latest)
}

// resolveCurrentVersion 确定当前运行版本。
// 优先取状态文件（升级链路的权威记录），其次尝试解析当前容器镜像 tag；两者都不可用时报错。
func (u *Upgrader) resolveCurrentVersion(state *State, info *runtime.ContainerInfo) (string, error) {
	if state != nil && state.Version != "" {
		return state.Version, nil
	}
	if info.Image != "" {
		if v, ok := versionFromImageTag(info.Image); ok {
			return v, nil
		}
		return "", fmt.Errorf("镜像(%s) tag 无法解析为版本号", info.Image)
	}
	return "", fmt.Errorf("容器镜像信息缺失")
}

// doUpgrade 拉取新镜像、登记待升级状态，并拉起升级执行器容器。
func (u *Upgrader) doUpgrade(ctx context.Context, rt runtime.Runtime, state *State, info *runtime.ContainerInfo, target string) error {
	targetImage := u.cfg.Image + ":" + target

	klog.Infof("拉取新版本镜像(%s)，视镜像体积可能耗时较长", targetImage)
	if err := rt.PullImage(ctx, targetImage); err != nil {
		return fmt.Errorf("拉取镜像(%s)失败: %v", targetImage, err)
	}

	spec := buildUpgradeSpec(u.cfg.ContainerName, info, targetImage)

	// 待升级状态先落盘：执行器与下一轮检查都依赖它
	state.Pending = &PendingUpgrade{
		TargetVersion: target,
		TargetImage:   targetImage,
		OldImage:      info.Image,
		Spec:          spec,
		RuntimeOpts:   u.rtOpts,
	}
	state.ContainerName = u.cfg.ContainerName
	state.UpdatedAt = time.Now()
	if err := u.saveState(state); err != nil {
		return fmt.Errorf("写入升级状态失败: %v", err)
	}

	// 执行器复用新镜像（保证二进制与新版本一致），挂载沿用原容器（含 /etc/pixiu 状态卷与 CRI socket）。
	// 放在独立 goroutine：RunContainer 会阻塞到执行器退出，而执行器删除旧容器会连带终止本进程，
	// 阻塞在本 job 内反而可能让 cron 记录出一条无意义的失败日志。
	args := []string{u.cfg.Executable, "upgrade-execute"}
	go func() {
		execCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		err := rt.RunContainer(execCtx, &runtime.ContainerSpec{
			Name:        u.cfg.ContainerName + "-upgrader",
			Image:       targetImage,
			Args:        args,
			Binds:       info.Binds,
			NetworkHost: true,
			Labels: map[string]string{
				"pixiu.io/role": "upgrade-executor",
			},
		})
		// 正常路径：执行器删除旧容器时本 goroutine 随宿主进程一起被终止，err 记录不到任何地方；
		// 这里的日志只在执行器自身启动/执行失败（旧容器未被删除，pixiu 未中断）时有意义
		if err != nil {
			klog.Errorf("升级执行器容器异常退出（pixiu 未被重建，将在下一轮检查中重试）: %v", err)
		}
	}()

	klog.Infof("已拉起升级执行器(%s-upgrader)，稍后将重建 pixiu 服务容器至 %s", u.cfg.ContainerName, target)
	return nil
}

// buildUpgradeSpec 由原容器检出信息构造新服务容器规格。
// 重启策略保留原容器的声明（原容器未声明自动拉起时不擅自添加）。
func buildUpgradeSpec(name string, info *runtime.ContainerInfo, targetImage string) *runtime.ServiceSpec {
	spec := &runtime.ServiceSpec{
		Name:          name,
		Image:         targetImage,
		Env:           info.Env,
		Binds:         info.Binds,
		NetworkHost:   info.NetworkHost,
		Privileged:    info.Privileged,
		RestartAlways: info.RestartPolicy == "always",
		Labels:        make(map[string]string, len(info.Labels)),
	}
	for k, v := range info.Labels {
		spec.Labels[k] = v
	}
	// 重启 monitor 相关标签由 RunServiceContainer 按新声明重建，不沿用旧值
	delete(spec.Labels, "containerd.io/restart.policy")
	delete(spec.Labels, "containerd.io/restart.status")
	delete(spec.Labels, "containerd.io/restart.count")
	delete(spec.Labels, "containerd.io/restart.explicitly-stopped")
	return spec
}

// loadState 读取升级状态；文件不存在或损坏时返回空状态（首次运行）。
func (u *Upgrader) loadState() *State {
	data, err := os.ReadFile(u.statePath())
	if err != nil {
		return &State{}
	}
	var state State
	if err = json.Unmarshal(data, &state); err != nil {
		klog.Errorf("升级状态文件(%s)解析失败，按空状态处理: %v", u.statePath(), err)
		return &State{}
	}
	return &state
}

// saveState 持久化升级状态（原子写，避免执行器读到半截 JSON）。
func (u *Upgrader) saveState(state *State) error {
	if err := os.MkdirAll(u.stateDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmp := u.statePath() + ".tmp"
	if err = os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, u.statePath())
}

// ExecuteUpgrade 升级执行器入口（pixiu-server upgrade-execute）。
// 以独立容器任务运行：删除旧 pixiu 容器 → 用新镜像重建服务容器 → 写回状态。
// 任一步骤失败时尝试用旧镜像回滚，保证服务尽量不中断在中间态。
func ExecuteUpgrade(ctx context.Context) error {
	return executeUpgrade(ctx, DefaultStateDir)
}

func executeUpgrade(ctx context.Context, stateDir string) error {
	statePath := filepath.Join(stateDir, stateFileName)
	data, err := os.ReadFile(statePath)
	if err != nil {
		return fmt.Errorf("读取升级状态失败: %v", err)
	}
	var state State
	if err = json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("升级状态解析失败: %v", err)
	}
	if state.Pending == nil {
		klog.Info("没有待应用的升级，执行器直接退出")
		return nil
	}
	pending := state.Pending

	klog.Infof("升级执行器启动，%s 后重建 pixiu 容器(%s)至版本 %s",
		executorStartupDelay, state.ContainerName, pending.TargetVersion)
	select {
	case <-time.After(executorStartupDelay):
	case <-ctx.Done():
		return ctx.Err()
	}

	rt, err := runtime.New(pending.RuntimeOpts)
	if err != nil {
		return markExecutorFailure(statePath, &state, fmt.Errorf("连接宿主容器运行时失败: %v", err))
	}

	// 删除旧容器：本执行器是宿主 CRI 上的独立任务，旧容器终止不影响自身
	klog.Infof("停止并删除旧 pixiu 容器(%s)", state.ContainerName)
	if err = rt.RemoveContainer(ctx, state.ContainerName); err != nil {
		return markExecutorFailure(statePath, &state, fmt.Errorf("删除旧容器失败: %v", err))
	}

	klog.Infof("以新镜像重建 pixiu 容器: %s", pending.TargetImage)
	if err = rt.RunServiceContainer(ctx, pending.Spec); err != nil {
		// 重建失败：用旧镜像尝试回滚，尽量不让服务停留在停止状态
		klog.Errorf("重建失败，尝试回滚到旧镜像(%s): %v", pending.OldImage, err)
		rollback := *pending.Spec
		rollback.Image = pending.OldImage
		if rbErr := rt.RunServiceContainer(ctx, &rollback); rbErr != nil {
			klog.Errorf("回滚也失败，pixiu 服务当前处于停止状态，需要人工介入: %v", rbErr)
		} else {
			klog.Info("已回滚到旧版本，服务恢复运行")
		}
		return markExecutorFailure(statePath, &state, fmt.Errorf("重建服务容器失败: %v", err))
	}

	// 成功：更新权威版本记录并清空待升级状态
	state.Version = pending.TargetVersion
	state.Image = pending.TargetImage
	state.Pending = nil
	state.UpdatedAt = time.Now()
	out, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmp := statePath + ".tmp"
	if err = os.WriteFile(tmp, out, 0o644); err != nil {
		return err
	}
	if err = os.Rename(tmp, statePath); err != nil {
		return err
	}

	klog.Infof("pixiu 自动升级完成: %s（镜像 %s）", pending.TargetVersion, pending.TargetImage)
	return nil
}

// markExecutorFailure 把执行器失败原因写回状态文件（供下一轮检查与人工排查）。
func markExecutorFailure(statePath string, state *State, execErr error) error {
	if state.Pending != nil {
		state.Pending.Error = execErr.Error()
	}
	state.UpdatedAt = time.Now()
	out, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return execErr
	}
	tmp := statePath + ".tmp"
	if err = os.WriteFile(tmp, out, 0o644); err != nil {
		return execErr
	}
	_ = os.Rename(tmp, statePath)
	return execErr
}
