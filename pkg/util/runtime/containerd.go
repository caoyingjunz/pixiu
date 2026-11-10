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
	"github.com/containerd/typeurl/v2"
	"github.com/distribution/reference"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"k8s.io/klog/v2"
)

const (
	// containerdStopGrace 优雅停止（SIGTERM）后等待进程退出的时间，超时强杀，对齐 docker Stop(5s)
	containerdStopGrace = 5 * time.Second
	// logPollInterval 跟随日志时的轮询间隔，避免忙等
	logPollInterval = 300 * time.Millisecond
	// nerdctlNameLabel 是 nerdctl 写入容器的名称标签；nerdctl 容器 ID 为哈希，名称仅存于标签。
	nerdctlNameLabel = "nerdctl/name"
)

// containerd 容器 ID 的合法字符集
var containerNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

// 日志路径约定（containerd 没有原生日志流，日志由 RunContainer 落盘到文件）：
//  1. 路径恒为 logPath(name) = <DefaultLogDir>/<name>.log（根目录固定不可配置），不接受调用方指定；
//  2. RunContainer 把该路径与 done 通道登记到 containerLogs，供同进程的 Logs 走快路径；
//  3. done 在 RunContainer 返回（日志文件已关闭）时关闭，follow 读取端据此及时 EOF，
//     避免容器退出后 SSE 流一直悬挂到客户端断开；
//  4. 登记表在 RunContainer 返回时注销。未命中登记表（pixiu 重启后/多实例）时，
//     Logs 直接用日志路径读取，follow 的收尾靠 runFinishedChecker（见其注释）。
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
	// socket 不可用时这里就会返回错误；懒加载下该错误只在首次实际使用（runtime.Default）时报出，
	// 不再阻断 pixiu 启动
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
	// 注：执行结束后容器与任务保留（见下方「保留容器」注释），关闭日志文件后，
	// cio 拷贝 goroutine 会在下一次写盘失败时自行退出。
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
		return fmt.Errorf("打开日志文件(%s)失败: %v；请确认日志目录可写（如 /etc/pixiu 已挂载进 pixiu 容器）", path, err)
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
		return err
	}

	if err = task.Start(waitCtx); err != nil {
		return err
	}

	var (
		waitErr  error
		exitCode uint32
	)
	select {
	case status := <-statusC:
		if err := status.Error(); err != nil {
			switch {
			case errors.Is(waitCtx.Err(), context.DeadlineExceeded):
				waitErr = fmt.Errorf("已等待 %d 秒，任务(%s)执行超时", int(waitTimeout.Seconds()), spec.Name)
			case waitCtx.Err() != nil:
				// 上层 ctx 被取消（如 agent 进程收到 SIGTERM），并非等待超时
				waitErr = fmt.Errorf("任务(%s)被取消: %v", spec.Name, waitCtx.Err())
			default:
				waitErr = err
			}
		} else {
			exitCode = status.ExitCode()
		}
	case <-waitCtx.Done():
		if errors.Is(waitCtx.Err(), context.DeadlineExceeded) {
			waitErr = fmt.Errorf("已等待 %d 秒，任务(%s)执行超时", int(waitTimeout.Seconds()), spec.Name)
		} else {
			waitErr = fmt.Errorf("任务(%s)被取消: %v", spec.Name, waitCtx.Err())
		}
	}

	// 执行结束后保留容器与任务（原清理逻辑已移除），与 docker 语义对齐：
	// docker 的 waitContainer 只等待与报错，从不删除容器，超时也不杀容器进程。
	// 失败/超时现场可继续用 `nerdctl ps -a` / `exec` / `inspect` 排查；
	// 同名容器由下一次执行的 RemoveContainer 统一清理（连同任务与 snapshot 回收）。

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

// resolveContainer 按名称解析 containerd 容器。
// pixiu 自行创建的容器 ID 即名称；nerdctl 创建的容器 ID 是哈希，名称保存在 nerdctl/name 标签中。
// 先按 ID 直查，未命中再按标签回退，兼容两种来源。
func (c *containerdRuntime) resolveContainer(ctx context.Context, name string) (containerd.Container, error) {
	container, err := c.client.LoadContainer(ctx, name)
	if err == nil {
		return container, nil
	}
	if !errdefs.IsNotFound(err) {
		return nil, err
	}

	containers, listErr := c.client.Containers(ctx)
	if listErr != nil {
		return nil, listErr
	}
	for _, candidate := range containers {
		info, infoErr := candidate.Info(ctx)
		if infoErr != nil {
			if errdefs.IsNotFound(infoErr) {
				continue
			}
			return nil, infoErr
		}
		if info.Labels[nerdctlNameLabel] == name {
			return candidate, nil
		}
	}
	return nil, err
}

// RemoveContainer 停止并删除同名容器；容器不存在时返回 nil
func (c *containerdRuntime) RemoveContainer(ctx context.Context, name string) error {
	container, err := c.resolveContainer(ctx, name)
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
	// 只有未命中登记表时才需要「运行是否已结束」这条兜底判据：命中时 done 已足够收尾，
	// 挂上探测反而会给每个正常日志流带来无谓的 gRPC 轮询
	var finished func(context.Context) (bool, error)
	if done == nil {
		finished = c.runFinishedChecker(name)
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
		return newFileFollower(ctx, path, done, nil, finished), nil
	}

	if !follow {
		return f, nil
	}
	return newFileFollower(ctx, path, done, f, finished), nil
}

// runFinishedChecker 返回「运行是否已结束」的探测函数，作为 follow 读取在未命中登记表
// （done 为 nil）时的结束判据。
// RunContainer 执行结束后保留容器与任务（对齐 docker 语义），容器记录不再随执行结束消失，
// 因此判据取任务状态而非容器存在性：
//   - 容器记录被清理（NotFound）→ 已结束（如被下一次同名执行的 RemoveContainer 回收）；
//   - 任务已停止（Stopped）或任务记录已被删除（NotFound）→ 已结束，日志已写完；
//   - 容器刚创建、任务尚未创建（Task 返回 NotFound）→ 未结束，继续跟随（避免误收尾）；
//   - 其余状态（Running/Paused/Created）→ 未结束。
//
// 探测出错时保守返回 (false, err)，交由 ctx 兜底（与改版前语义一致）。
// ⚠️ 对偶提醒：若将来改回「执行结束后删除容器」，本判据的容器缺失分支将重新成为主判据，
// 必须同步复核此处，避免日志流悬挂到客户端断开。
func (c *containerdRuntime) runFinishedChecker(name string) func(context.Context) (bool, error) {
	return func(ctx context.Context) (bool, error) {
		container, err := c.client.LoadContainer(ctx, name)
		if err != nil {
			if errdefs.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}

		task, err := container.Task(ctx, nil)
		if err != nil {
			if errdefs.IsNotFound(err) {
				// 容器刚创建、任务尚未创建：继续跟随
				return false, nil
			}
			return false, err
		}

		status, err := task.Status(ctx)
		if err != nil {
			if errdefs.IsNotFound(err) {
				// 任务记录已被删除：执行已结束
				return true, nil
			}
			return false, err
		}
		return status.Status == containerd.Stopped, nil
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

// fileFollower 以轮询方式跟随仍在增长的日志文件。
// 与 docker 原生 follow 不同，这里只能靠外部信号判断结束，有两条判据：
//  1. done（RunContainer 返回、日志文件已关闭时关闭）——同进程快路径；
//  2. runFinished（运行已结束：任务已停止或容器记录被清理）——未命中登记表时的兜底，见 runFinishedChecker。
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
	// runFinished 探测运行是否已结束；done 为 nil 时的收尾判据
	runFinished func(context.Context) (bool, error)
}

func newFileFollower(ctx context.Context, path string, done <-chan struct{}, f *os.File, finished func(context.Context) (bool, error)) *fileFollower {
	pipeR, pipeW := io.Pipe()
	l := &fileFollower{
		ctx:         ctx,
		path:        path,
		done:        done,
		file:        f,
		pipeR:       pipeR,
		pipeW:       pipeW,
		closed:      make(chan struct{}),
		runFinished: finished,
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
		if l.runFinished != nil {
			// 探测失败（网络/权限等）时保守地继续跟随，交由 ctx 兜底
			if finished, err := l.runFinished(l.ctx); err == nil && finished {
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

// 自升级场景的容器操作（仅本地部署使用）。

// upgradeLogFiles 保存升级重建任务日志文件句柄，避免被 GC 回收导致容器输出写失败。
var upgradeLogFiles sync.Map

// upgradeJSONLogFiles 保存升级重建任务的 nerdctl JSON 日志文件句柄，避免被 GC 回收。
var upgradeJSONLogFiles sync.Map

// nerdctlLogWriter 将容器输出编码为 nerdctl json-file 驱动的日志行。
// nerdctl logs 读取 <state-dir>/<容器名>-json.log，因此升级重建的容器必须同步写入该文件。
type nerdctlLogWriter struct {
	file   *os.File
	stream string
	mu     *sync.Mutex
}

func (w *nerdctlLogWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	entry, err := json.Marshal(struct {
		Log    string `json:"log"`
		Stream string `json:"stream"`
		Time   string `json:"time"`
	}{
		Log:    string(p),
		Stream: w.stream,
		Time:   time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return 0, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, err = w.file.Write(append(entry, '\n')); err != nil {
		return 0, err
	}
	return len(p), nil
}

// prepareNerdctlLog 为 containerd 直接重建的容器补齐 nerdctl 状态目录与 JSON 日志。
// oldStateDir 取自原容器的 nerdctl/state-dir 标签；父目录不存在（未挂载 /var/lib/nerdctl）
// 时返回 nil，调用方回退为仅写普通日志文件。
func prepareNerdctlLog(name, oldStateDir string) (io.Writer, io.Writer, *os.File, error) {
	if oldStateDir == "" {
		return nil, nil, nil, nil
	}
	parent := filepath.Dir(oldStateDir)
	if _, err := os.Stat(parent); err != nil {
		klog.Warningf("nerdctl 状态目录不可用（%s），升级后 nerdctl logs 将不可用: %v", parent, err)
		return nil, nil, nil, nil
	}

	newStateDir := filepath.Join(parent, name)
	if err := os.MkdirAll(newStateDir, 0o700); err != nil {
		return nil, nil, nil, err
	}
	for _, file := range []string{"log-config.json", "hostname", "network-config.json", "resolv.conf"} {
		if data, err := os.ReadFile(filepath.Join(oldStateDir, file)); err == nil {
			_ = os.WriteFile(filepath.Join(newStateDir, file), data, 0o600)
		}
	}
	if _, err := os.Stat(filepath.Join(newStateDir, "log-config.json")); err != nil {
		cfg := `{"driver":"json-file","address":"/run/containerd/containerd.sock"}`
		if err := os.WriteFile(filepath.Join(newStateDir, "log-config.json"), []byte(cfg), 0o600); err != nil {
			return nil, nil, nil, err
		}
	}
	if lock, err := os.OpenFile(filepath.Join(newStateDir, "logger-lock"), os.O_CREATE|os.O_RDWR, 0o600); err == nil {
		_ = lock.Close()
	}
	logFile, err := os.OpenFile(filepath.Join(newStateDir, name+"-json.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, nil, nil, err
	}
	mu := &sync.Mutex{}
	return &nerdctlLogWriter{file: logFile, stream: "stdout", mu: mu},
		&nerdctlLogWriter{file: logFile, stream: "stderr", mu: mu},
		logFile, nil
}

// ContainerImageRef 返回容器当前使用的镜像引用。
func (c *containerdRuntime) ContainerImageRef(ctx context.Context, name string) (string, error) {
	container, err := c.resolveContainer(ctx, name)
	if err != nil {
		return "", err
	}
	info, err := container.Info(ctx)
	if err != nil {
		return "", err
	}
	if info.Image == "" {
		return "", fmt.Errorf("容器(%s)缺少镜像信息", name)
	}
	return info.Image, nil
}

// IsContainerRunning 返回容器是否处于运行状态；容器不存在时返回 false, nil。
func (c *containerdRuntime) IsContainerRunning(ctx context.Context, name string) (bool, error) {
	container, err := c.resolveContainer(ctx, name)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	task, err := container.Task(ctx, nil)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	status, err := task.Status(ctx)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return status.Status == containerd.Running, nil
}

// ReplaceContainer 使用 containerd 客户端按原容器配置用 newImage 重建容器，返回旧镜像引用。
// 复用原容器的 OCI spec（env/mounts/network/privileged 等）与 labels（含 restart 策略），
// 不依赖 nerdctl。
func (c *containerdRuntime) ReplaceContainer(ctx context.Context, name, newImage string) (string, error) {
	container, err := c.resolveContainer(ctx, name)
	if err != nil {
		return "", err
	}
	info, err := container.Info(ctx)
	if err != nil {
		return "", err
	}
	oldImage := info.Image
	if info.Spec == nil {
		return oldImage, fmt.Errorf("容器(%s)缺少 OCI spec，无法重建", name)
	}
	spec := &oci.Spec{}
	if err = typeurl.UnmarshalTo(info.Spec, spec); err != nil {
		return oldImage, fmt.Errorf("解析容器(%s) OCI spec 失败: %v", name, err)
	}

	imageRef, err := normalizeImageRef(newImage)
	if err != nil {
		return oldImage, fmt.Errorf("非法的镜像名(%s): %v", newImage, err)
	}
	// 先拉镜像成功，再删除旧容器，避免拉取失败导致服务中断
	if err = c.PullImage(ctx, imageRef); err != nil {
		return oldImage, err
	}
	img, err := c.client.GetImage(ctx, imageRef)
	if err != nil {
		return oldImage, err
	}
	if err = c.RemoveContainer(ctx, name); err != nil {
		return oldImage, err
	}

	// 升级重建的容器由 containerd 直接创建，不会生成 nerdctl 的容器状态目录；
	// 此处补齐状态目录与 json-file 日志，保证升级后 nerdctl logs 仍可用。
	labels := make(map[string]string, len(info.Labels)+1)
	for k, v := range info.Labels {
		labels[k] = v
	}
	oldStateDir := labels["nerdctl/state-dir"]
	stdoutLogWriter, stderrLogWriter, jsonLogFile, prepErr := prepareNerdctlLog(name, oldStateDir)
	if prepErr != nil {
		klog.Warningf("初始化 nerdctl 日志状态失败，升级后 nerdctl logs 将不可用: %v", prepErr)
	}
	if jsonLogFile != nil {
		labels["nerdctl/state-dir"] = filepath.Join(filepath.Dir(oldStateDir), name)
		if old, ok := upgradeJSONLogFiles.Load(name); ok {
			if file, ok := old.(*os.File); ok {
				_ = file.Close()
			}
		}
		upgradeJSONLogFiles.Store(name, jsonLogFile)
	}

	// 普通日志文件仍保留（/etc/pixiu/runner-logs/<name>.log），便于运维直接查看。
	plainLogFile, err := openLogFile(logPath(name))
	if err != nil {
		if jsonLogFile != nil {
			_ = jsonLogFile.Close()
		}
		return oldImage, err
	}
	if old, ok := upgradeLogFiles.Load(name); ok {
		if file, ok := old.(*os.File); ok {
			_ = file.Close()
		}
	}
	upgradeLogFiles.Store(name, plainLogFile)

	var stdoutWriter io.Writer = plainLogFile
	var stderrWriter io.Writer = plainLogFile
	if stdoutLogWriter != nil {
		stdoutWriter = io.MultiWriter(plainLogFile, stdoutLogWriter)
		stderrWriter = io.MultiWriter(plainLogFile, stderrLogWriter)
	}

	containerOpts := []containerd.NewContainerOpts{
		containerd.WithImage(img),
		containerd.WithNewSnapshot(name, img),
		containerd.WithSpec(spec),
	}
	if len(labels) > 0 {
		containerOpts = append(containerOpts, containerd.WithContainerLabels(labels))
	}
	newContainer, err := c.client.NewContainer(ctx, name, containerOpts...)
	if err != nil {
		if jsonLogFile != nil {
			_ = jsonLogFile.Close()
		}
		return oldImage, err
	}

	task, err := newContainer.NewTask(ctx, cio.NewCreator(cio.WithStreams(nil, stdoutWriter, stderrWriter)))
	if err != nil {
		if jsonLogFile != nil {
			_ = jsonLogFile.Close()
		}
		return oldImage, err
	}
	if err = task.Start(ctx); err != nil {
		return oldImage, err
	}
	return oldImage, nil
}
