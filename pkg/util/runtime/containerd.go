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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/distribution/reference"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"k8s.io/klog/v2"
)

const (
	// containerdStopGrace 优雅停止（SIGTERM）后等待进程退出的时间，超时强杀，对齐 docker Stop(5s)
	containerdStopGrace = 5 * time.Second
	// containerdCleanupTimeout 清理阶段（强杀任务 + 删除容器 + 回收 snapshot）的独立超时。
	// 清理不能复用调用方 ctx：调用方 ctx 超时/取消正是最需要清理的场景。
	containerdCleanupTimeout = 30 * time.Second
	// logPollInterval 跟随日志时的轮询间隔，避免忙等
	logPollInterval = 300 * time.Millisecond
)

// containerd 容器 ID 的合法字符集
var containerNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

// 日志路径约定（containerd 没有原生日志流，日志由 RunContainer 落盘到文件）：
//  1. 路径恒为 logPath(name) = <DefaultLogDir>/<name>.log（根目录固定不可配置），不接受调用方指定；
//  2. RunContainer 把该路径与 done 通道登记到 containerLogs，供同进程的 Logs 走快路径；
//  3. done 在 RunContainer 返回（日志文件已关闭）时关闭，follow 读取端据此及时 EOF，
//     避免容器退出后 SSE 流一直悬挂到客户端断开；
//  4. 登记表在 RunContainer 返回时注销。未命中登记表（pixiu 重启后/多实例）时，
//     Logs 直接用日志路径读取，follow 的收尾靠 goneChecker（见其注释）。
type logEntry struct {
	path string
	done chan struct{}
}

var containerLogs sync.Map // 容器名 -> *logEntry

type containerdRuntime struct {
	client *containerd.Client
}

func newContainerdRuntime(opts config.RuntimeOptions) (*containerdRuntime, error) {
	address := opts.Socket
	if address == "" {
		return nil, fmt.Errorf("containerd 运行时缺少 socket 路径配置(runtime.socket)")
	}

	// containerd.New 内部用 grpc.WithBlock + 10s 超时同步拨号（containerd v1.7 client.go），
	// socket 不可用时这里就会返回错误，因此本函数（进而 Init）确实是 fail-fast
	// namespace 不再暴露为配置：统一交给 containerd 客户端的默认机制落位，
	// 即 namespaces.Default（"default"，与 nerdctl/ctr 默认命名空间一致）
	cli, err := containerd.New(address, containerd.WithDefaultNamespace(namespaces.Default))
	if err != nil {
		return nil, fmt.Errorf("连接 containerd 失败（%s）：%v；请确认 containerd 正在运行、socket 已挂载进 pixiu 容器，或将 runtime.cri 显式配置为 docker", address, err)
	}

	return &containerdRuntime{client: cli}, nil
}

func (c *containerdRuntime) Kind() Kind {
	return KindContainerd
}

func (c *containerdRuntime) Close() error {
	return c.client.Close()
}

// RunContainer 清理同名旧容器 → 创建 → 启动 → 等待退出
// 与 docker 实现的语义差异：containerd 没有原生日志流，stdout/stderr 必须先落盘到
// logPath(spec.Name)，供并发的 Logs(follow) 读取
func (c *containerdRuntime) RunContainer(ctx context.Context, spec *ContainerSpec) error {
	if spec.Image == "" {
		return fmt.Errorf("容器(%s)未指定镜像", spec.Name)
	}
	if !containerNameRe.MatchString(spec.Name) {
		return fmt.Errorf("非法的容器名(%s)，只允许字母、数字、下划线、点和中划线，且必须以字母或数字开头", spec.Name)
	}
	imageRef, err := normalizeImageRef(spec.Image)
	if err != nil {
		return fmt.Errorf("非法的镜像名(%s): %v", spec.Image, err)
	}

	// 已经存在，则先删除运行的容器
	if err := c.RemoveContainer(ctx, spec.Name); err != nil {
		return err
	}

	// 日志路径只由容器名决定（根目录固定 DefaultLogDir），不接受调用方指定（见 logPath）
	path := logPath(spec.Name)
	entry := &logEntry{path: path, done: make(chan struct{})}
	containerLogs.Store(spec.Name, entry)
	// 登记表的注销必须覆盖本函数的所有返回路径：提前 return 若漏掉，follow 读取端会永远
	// 等不到 done，登记表也会随运行次数单调增长。
	// 顺序重要：先关日志文件 → 再关 done 通知读取端收尾 → 最后注销登记。
	// 注：正常/超时路径返回前已删除任务与容器，cio 拷贝 goroutine 随之收尾；取消路径容器
	// 可能仍在运行，关闭日志文件会让拷贝 goroutine 在下一次写入失败后自行退出（见 canceled 分支）。
	var logFile *os.File
	defer func() {
		if logFile != nil {
			_ = logFile.Close()
		}
		close(entry.done)
		containerLogs.CompareAndDelete(spec.Name, entry)
	}()

	logFile, err = openLogFile(path)
	if err != nil {
		return fmt.Errorf("打开日志文件(%s)失败: %v；请确认日志目录可写（如 /var/lib/pixiu 已挂载进 pixiu 容器）", path, err)
	}

	img, err := c.client.GetImage(ctx, imageRef)
	if err != nil {
		return err
	}

	specOpts, err := buildSpecOpts(spec, img)
	if err != nil {
		return err
	}

	containerOpts := []containerd.NewContainerOpts{
		containerd.WithImage(img),
		containerd.WithNewSnapshot(spec.Name, img),
		containerd.WithNewSpec(specOpts...),
	}
	// 容器元数据与 docker 实现对齐（author/pixiuName 等）；
	// 注意 containerd 1.7 对应的选项是 WithContainerLabels，不存在 WithLabels
	if len(spec.Labels) > 0 {
		containerOpts = append(containerOpts, containerd.WithContainerLabels(spec.Labels))
	}

	container, err := c.client.NewContainer(ctx, spec.Name, containerOpts...)
	if err != nil {
		if !errdefs.IsAlreadyExists(err) {
			return err
		}
		// 仅当容器记录确不存在（纯 snapshot 残留）才自愈；记录仍在属并发同名等场景，
		// 此时 snapshot 为在用状态，贸然删除会破坏对方容器，直接返回原错误。
		if _, loadErr := c.client.LoadContainer(ctx, spec.Name); !errdefs.IsNotFound(loadErr) {
			return err
		}
		// 罕见场景：容器记录已删除但 snapshot 残留（如异常终止），同名 snapshot 会让该任务此后
		// 永远无法创建；此处按与库内一致的解析顺序定位 snapshotter，清掉残留后重试一次。
		if cleanErr := c.removeOrphanSnapshot(ctx, spec.Name); cleanErr != nil {
			klog.Warningf("清理残留 snapshot(%s)失败: %v", spec.Name, cleanErr)
		}
		container, err = c.client.NewContainer(ctx, spec.Name, containerOpts...)
		if err != nil {
			return err
		}
	}

	task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStreams(nil, logFile, logFile)))
	if err != nil {
		c.cleanupTaskAndContainer(spec.Name, nil, container, false)
		return err
	}

	// 等待上限：>0 取调用方配置（worker.deploy_timeout，经 ContainerSpec.WaitTimeout 传入），
	// 到点即判定超时；<=0 表示不设内部上限（等待容器退出或 ctx 结束）。
	// 若调用方 ctx 自带更早的截止时间，则取其剩余时长，保证不超时。
	waitTimeout := spec.WaitTimeout
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 && (waitTimeout <= 0 || remaining < waitTimeout) {
			waitTimeout = remaining
		}
	}

	waitCtx := ctx
	cancel := func() {}
	if waitTimeout > 0 {
		waitCtx, cancel = context.WithTimeout(ctx, waitTimeout)
		klog.Infof("等待任务(%s)执行完成(最长等待时间 %d 秒)", spec.Name, int(waitTimeout.Seconds()))
	} else {
		klog.Infof("等待任务(%s)执行完成(不限制等待时长)", spec.Name)
	}
	defer cancel()

	// Wait 必须在 Start 之前注册，否则可能错过退出事件。
	// 注意：containerd 1.7 的 task.Wait 返回 (<-chan ExitStatus, error)，
	// 通道内的事件在等待出错时会带上 err（含 ctx 超时导致的 RPC 取消），退出码不可直接采信
	statusC, err := task.Wait(waitCtx)
	if err != nil {
		c.cleanupTaskAndContainer(spec.Name, task, container, true)
		return err
	}

	if err = task.Start(waitCtx); err != nil {
		c.cleanupTaskAndContainer(spec.Name, task, container, true)
		return err
	}

	var (
		waitErr  error
		exitCode uint32
		exited   bool
		canceled bool
	)
	select {
	case status := <-statusC:
		if err := status.Error(); err != nil {
			switch {
			case errors.Is(waitCtx.Err(), context.DeadlineExceeded):
				waitErr = fmt.Errorf("已等待 %d 秒，任务(%s)执行超时", int(waitTimeout.Seconds()), spec.Name)
			case waitCtx.Err() != nil:
				// 上层 ctx 被取消（如 agent 进程收到 SIGTERM），并非等待超时
				canceled = true
				waitErr = fmt.Errorf("任务(%s)被取消: %v", spec.Name, waitCtx.Err())
			default:
				waitErr = err
			}
		} else {
			exited = true
			exitCode = status.ExitCode()
		}
	case <-waitCtx.Done():
		if errors.Is(waitCtx.Err(), context.DeadlineExceeded) {
			waitErr = fmt.Errorf("已等待 %d 秒，任务(%s)执行超时", int(waitTimeout.Seconds()), spec.Name)
		} else {
			canceled = true
			waitErr = fmt.Errorf("任务(%s)被取消: %v", spec.Name, waitCtx.Err())
		}
	}

	if canceled {
		// 取消（非超时）不杀容器：与升级前 agent 行为一致，runner 容器可继续完成部署，
		// 残留的容器记录由下一次同名任务的 RemoveContainer 回收。
		return waitErr
	}

	// 无论成功失败都要删除任务与容器；必须用独立清理上下文（此刻调用方 ctx 可能已超时/取消），
	// WithSnapshotCleanup 必须带，否则 snapshot 残留会持续占用宿主磁盘
	c.cleanupTaskAndContainer(spec.Name, task, container, !exited)

	if waitErr != nil {
		return waitErr
	}
	if exitCode != 0 {
		return fmt.Errorf("容器(%s)执行失败，退出码 %d", spec.Name, exitCode)
	}
	return nil
}

// removeOrphanSnapshot 删除与容器名同 key 的残留 snapshot（不含容器记录的场景）。
// snapshotter 名称解析顺序与 containerd 客户端内部一致：命名空间 label 优先，其次内置默认。
func (c *containerdRuntime) removeOrphanSnapshot(ctx context.Context, key string) error {
	snapshotter := containerd.DefaultSnapshotter
	if label, err := c.client.GetLabel(ctx, defaults.DefaultSnapshotterNSLabel); err == nil && label != "" {
		snapshotter = label
	}
	return c.client.SnapshotService(snapshotter).Remove(ctx, key)
}

// cleanupTaskAndContainer 删除任务与容器记录（含 snapshot）。
// 刻意切到独立的 background 上下文：执行超时/被取消时调用方 ctx 已失效，复用它会让
// Kill/Delete 全部失败，留下仍在运行的容器与被占用的 snapshot（下一次同名任务虽能自愈，
// 但容器不落日志地一直跑本身就是事故）。
func (c *containerdRuntime) cleanupTaskAndContainer(name string, task containerd.Task, container containerd.Container, running bool) {
	ctx, cancel := context.WithTimeout(context.Background(), containerdCleanupTimeout)
	defer cancel()

	if task != nil {
		if err := deleteTask(ctx, task, running); err != nil {
			klog.Warningf("清理任务(%s)失败: %v", name, err)
		}
	}
	if err := container.Delete(ctx, containerd.WithSnapshotCleanup); err != nil {
		klog.Warningf("清理容器(%s)失败: %v", name, err)
	}
}

// RemoveContainer 停止并删除同名容器；容器不存在时返回 nil
func (c *containerdRuntime) RemoveContainer(ctx context.Context, name string) error {
	container, err := c.client.LoadContainer(ctx, name)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}
		return err
	}

	// 容器记录存在但可能没有运行中的 task（创建后未启动 / 已退出未删除）
	task, err := container.Task(ctx, nil)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return err
		}
		task = nil
	}
	if task != nil {
		// 优雅停止，超时强杀，等价 docker 的 Stop(5s) + Force Remove
		statusC, _ := task.Wait(ctx)
		_ = task.Kill(ctx, syscall.SIGTERM)
		select {
		case <-statusC:
		case <-time.After(containerdStopGrace):
			_ = task.Kill(ctx, syscall.SIGKILL)
			select {
			case <-statusC:
			case <-time.After(containerdStopGrace):
			}
		}
		if _, err = task.Delete(ctx); err != nil && !errdefs.IsNotFound(err) {
			return err
		}
	}

	return container.Delete(ctx, containerd.WithSnapshotCleanup)
}

// Logs 返回容器日志读取流。
// containerd 没有等价于 docker ContainerLogs(follow) 的接口，日志来源是 RunContainer 落盘的文件。
// 路径优先取 containerLogs 登记表（同进程快路径，附带 done 收尾信号）；未命中——pixiu 重启后、
// 多实例、或 SSE 早于 RunContainer 登记——则直接用 logPath(name) 这个确定性路径，不再靠推导猜。
func (c *containerdRuntime) Logs(ctx context.Context, name string, follow bool) (io.ReadCloser, error) {
	path := logPath(name)
	var done <-chan struct{}
	if v, ok := containerLogs.Load(name); ok {
		entry := v.(*logEntry)
		path, done = entry.path, entry.done
	}
	// 只有未命中登记表时才需要「容器消失」这条兜底判据：命中时 done 已足够收尾，
	// 挂上探测反而会给每个正常日志流带来无谓的 gRPC 轮询
	var gone func(context.Context) (bool, error)
	if done == nil {
		gone = c.goneChecker(name)
	}

	f, err := os.Open(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		if !follow {
			return nil, fmt.Errorf("容器(%s)不存在或日志(%s)尚未生成", name, path)
		}
		// follow 场景下文件可能尚未落盘（容器刚创建），交给 follower 轮询等待
		return newFileFollower(ctx, path, done, nil, gone), nil
	}

	if !follow {
		return f, nil
	}
	return newFileFollower(ctx, path, done, f, gone), nil
}

// goneChecker 返回「容器记录是否已被删除」的探测函数，作为 follow 读取在未命中登记表
// （done 为 nil）时的结束判据。
// RunContainer 除「上层 ctx 被取消」外的所有退出路径都会删除容器记录
// （cleanupTaskAndContainer），因此容器记录消失即可判定日志已写完。
// 取消路径有意保留容器记录（容器可能仍在运行，由下一次同名任务回收），此时未命中登记表的
// follow 无法靠 goneChecker 自然收尾，只能等客户端断开或 ctx 结束；该场景在服务端本地模式
// 不会出现（本地 ctx 只有截止时间、无取消源），agent 侧不使用 follow。
// ⚠️ 若将来让 RunContainer 在其他场景也保留容器记录（例如为排障保留现场），必须同步修改这里，
// 否则日志流只能等到客户端断开或 ctx 结束，SSE 会一直挂着。
func (c *containerdRuntime) goneChecker(name string) func(context.Context) (bool, error) {
	return func(ctx context.Context) (bool, error) {
		if _, err := c.client.LoadContainer(ctx, name); err != nil {
			if errdefs.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		return false, nil
	}
}

func (c *containerdRuntime) ImageExists(ctx context.Context, ref string) (bool, error) {
	imageRef, err := normalizeImageRef(ref)
	if err != nil {
		return false, err
	}

	if _, err := c.client.GetImage(ctx, imageRef); err != nil {
		if errdefs.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *containerdRuntime) PullImage(ctx context.Context, ref string) error {
	imageRef, err := normalizeImageRef(ref)
	if err != nil {
		return err
	}

	// WithPullUnpack 必须带：不解包则后续 WithNewSnapshot 无法创建 snapshot
	img, err := c.client.Pull(ctx, imageRef, containerd.WithPullUnpack)
	if err != nil {
		return err
	}

	klog.Infof("镜像(%s)拉取完成，digest: %s", imageRef, img.Target().Digest)
	return nil
}

func (c *containerdRuntime) RemoveImage(ctx context.Context, ref string) error {
	imageRef, err := normalizeImageRef(ref)
	if err != nil {
		return err
	}

	return c.client.ImageService().Delete(ctx, imageRef)
}

// normalizeImageRef 把镜像名归一化为 containerd 能解析的规范引用，对齐 docker 的短名语义。
// containerd 不会像 docker 那样补全短名：registry 引用解析（containerd/reference.Parse）把
// 首段当作 host，且要求必须有 tag 或 digest，于是 "runner:v1" 直接解析报错（invalid port），
// "library/runner:v1" 会被解析成去访问名为 library 的 registry（DNS 必失败），"runner" 则在
// Resolve 阶段因 object 为空返回 ErrObjectRequired（remotes/docker/resolver.go）。
// 因此无 registry host 时补 docker.io/、无路径时补 library/、无 tag/digest 时补 :latest。
// 四个镜像入口（RunContainer/ImageExists/PullImage/RemoveImage）必须共用本函数，否则会出现
// 「拉取用 A 名字、查询用 B 名字」而查不到镜像。
func normalizeImageRef(ref string) (string, error) {
	if ref == "" {
		return "", fmt.Errorf("镜像名为空")
	}

	named, err := reference.ParseNormalizedNamed(ref)
	if err != nil {
		return "", err
	}

	return reference.TagNameOnly(named).String(), nil
}

func openLogFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
}

// buildSpecOpts 把与运行时无关的 ContainerSpec 转成 containerd 的 OCI spec 选项。
// WithNewSpec 内部经 GenerateSpec 已生成默认 spec（命名空间/挂载/能力等），无需再叠加 oci.WithDefaultSpec；
// 但该默认 spec 的设备 cgroup 规则是 deny-all，必须显式放行 /dev/null、/dev/random 等基础设备，
// 否则 runner 容器内 ssh/ansible 重定向 /dev/null 会 EPERM（ctr run 亦显式带 WithDefaultUnixDevices）
func buildSpecOpts(spec *ContainerSpec, img containerd.Image) ([]oci.SpecOpts, error) {
	opts := []oci.SpecOpts{oci.WithImageConfig(img), oci.WithDefaultUnixDevices}

	if len(spec.Env) > 0 {
		opts = append(opts, oci.WithEnv(spec.Env))
	}
	if spec.NetworkHost {
		opts = append(opts, oci.WithHostNamespace(specs.NetworkNamespace))
	}
	if len(spec.Binds) > 0 {
		mounts, err := parseBinds(spec.Binds)
		if err != nil {
			return nil, err
		}
		opts = append(opts, oci.WithMounts(mounts))
	}
	return opts, nil
}

// parseBinds 把 docker 风格的 host:container 挂载串转成 OCI mount
func parseBinds(binds []string) ([]specs.Mount, error) {
	mounts := make([]specs.Mount, 0, len(binds))
	for _, bind := range binds {
		parts := strings.SplitN(bind, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("非法的挂载配置(%s)，期望格式 host:container", bind)
		}
		mounts = append(mounts, specs.Mount{
			Destination: parts[1],
			Type:        "bind",
			Source:      parts[0],
			Options:     []string{"rbind", "rw"},
		})
	}
	return mounts, nil
}

// deleteTask 删除任务记录；任务仍在运行时先强杀，否则 Delete 会失败导致任务残留
func deleteTask(ctx context.Context, task containerd.Task, running bool) error {
	if running {
		// 先注册 Wait 再 Kill，避免错过退出事件
		statusC, _ := task.Wait(ctx)
		_ = task.Kill(ctx, syscall.SIGKILL)
		select {
		case <-statusC:
		case <-time.After(containerdStopGrace):
		case <-ctx.Done():
		}
	}
	if _, err := task.Delete(ctx); err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	return nil
}

// fileFollower 以轮询方式跟随仍在增长的日志文件。
// 与 docker 原生 follow 不同，这里只能靠外部信号判断结束，有两条判据：
//  1. done（RunContainer 返回、日志文件已关闭时关闭）——同进程快路径；
//  2. containerGone（容器记录已被删除）——未命中登记表时的兜底，见 goneChecker。
//
// 两者都不可用时（理论上不会发生）只能等 ctx 结束。
type fileFollower struct {
	ctx    context.Context
	path   string
	done   <-chan struct{}
	file   *os.File
	pipeR  *io.PipeReader
	pipeW  *io.PipeWriter
	closed chan struct{}
	once   sync.Once
	// containerGone 探测容器记录是否已消失；done 为 nil 时的收尾判据
	containerGone func(context.Context) (bool, error)
}

func newFileFollower(ctx context.Context, path string, done <-chan struct{}, f *os.File, gone func(context.Context) (bool, error)) *fileFollower {
	pipeR, pipeW := io.Pipe()
	l := &fileFollower{
		ctx:           ctx,
		path:          path,
		done:          done,
		file:          f,
		pipeR:         pipeR,
		pipeW:         pipeW,
		closed:        make(chan struct{}),
		containerGone: gone,
	}
	go l.pump()
	return l
}

// pump 独占 file 字段：打开、读取、关闭全部在本 goroutine 内完成，
// 其他 goroutine（尤其 Close）不得触碰 l.file，避免数据竞争。
func (l *fileFollower) pump() {
	// 关闭顺序：先关管道读端侧（通知调用方结束），再关文件
	defer func() {
		if l.file != nil {
			_ = l.file.Close()
		}
	}()
	defer l.pipeW.Close()

	ticker := time.NewTicker(logPollInterval)
	defer ticker.Stop()

	for {
		// 文件尚未落盘时轮询等待出现
		if l.file == nil {
			f, err := os.Open(l.path)
			if err != nil {
				if !os.IsNotExist(err) {
					_ = l.pipeW.CloseWithError(err)
					return
				}
				if !l.wait(ticker) {
					return
				}
				continue
			}
			l.file = f
		}

		if _, err := io.Copy(l.pipeW, l.file); err != nil {
			_ = l.pipeW.CloseWithError(err)
			return
		}

		// 已读到当前末尾，等待新内容或结束信号
		if !l.wait(ticker) {
			// 结束前做最后一次读取，尽量把尾部内容带走
			if _, err := io.Copy(l.pipeW, l.file); err != nil {
				_ = l.pipeW.CloseWithError(err)
			}
			return
		}
	}
}

// wait 阻塞直到「轮询间隔到点，可能有新内容可读」或「应结束日志流」。
// 返回 true 表示可以继续读取（pump 将重试打开文件或读取新增内容），false 表示应结束。
func (l *fileFollower) wait(ticker *time.Ticker) bool {
	select {
	case <-l.ctx.Done():
		return false
	case <-l.closed:
		return false
	case <-l.done: // done 为 nil（未命中登记表）时该分支永不触发
		return false
	case <-ticker.C:
		if l.containerGone != nil {
			// 探测失败（网络/权限等）时保守地继续跟随，交由 ctx 兜底
			if gone, err := l.containerGone(l.ctx); err == nil && gone {
				return false
			}
		}
		// 轮询到点：文件可能刚落盘或已有新增内容，回到 pump 重试读取
		return true
	}
}

func (l *fileFollower) Read(p []byte) (int, error) {
	return l.pipeR.Read(p)
}

// Close 只负责信号与屏障：通知 pump 结束并关闭读端；file 由 pump 自行关闭（见 pump），
// 本方法刻意不触碰 l.file，否则会与 pump 并发读写该字段。
func (l *fileFollower) Close() error {
	l.once.Do(func() {
		close(l.closed)
		_ = l.pipeR.Close()
	})
	return nil
}

var _ Runtime = (*containerdRuntime)(nil)
