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

package cicd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

const (
	pendingBatchSize = 20
	logPollInterval  = 500 * time.Millisecond
)

// Run 启动 CICD 编排 worker 池。启动阶段先按集群事实源对账 running 记录，
// 再起 workers 个 goroutine 轮询 pending。ctx 取消时退出。
func (c *controller) Run(ctx context.Context, workers int) error {
	if !c.cc.CICD.Enabled {
		klog.Info("CICD is disabled by config, worker not started")
		return nil
	}
	if workers <= 0 {
		workers = c.workerCount()
	}

	// 启动对账：以集群事实源为准收口 running 记录。
	c.reconcileOnStartup(ctx)

	klog.Infof("starting CICD pipeline manager with %d worker(s)", workers)
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.worker, c.pollInterval())
	}
	return nil
}

// reconcileOnStartup 处理上次进程异常中断遗留的 running 记录：
// 1) 标记长时间未更新的为 failed；2) 回查集群 PipelineRun 真实状态收口。
func (c *controller) reconcileOnStartup(ctx context.Context) {
	cutoff := time.Now().Add(-10 * time.Minute)
	if n, err := c.factory.CICD().MarkStaleRunningFailed(ctx, cutoff); err != nil {
		klog.Errorf("[cicd] failed to mark stale running runs: %v", err)
	} else if n > 0 {
		klog.Infof("[cicd] marked %d stale running run(s) as failed", n)
	}

	runs, err := c.factory.CICD().ListRunningRuns(ctx, 200)
	if err != nil {
		klog.Errorf("[cicd] failed to list running runs for reconcile: %v", err)
		return
	}
	for i := range runs {
		c.syncRunStatus(ctx, &runs[i])
	}
}

// worker 单轮：捞取 pending run 并逐个原子 claim 执行。
func (c *controller) worker(ctx context.Context) {
	records, err := c.factory.CICD().ListPendingRuns(ctx, pendingBatchSize)
	if err != nil {
		klog.Errorf("[cicd] failed to list pending runs: %v", err)
		return
	}
	for i := range records {
		rec := &records[i]
		// 单流水线并发限制：claim 前再校验一次。
		running, err := c.factory.CICD().CountRunningByPipeline(ctx, rec.PipelineId)
		if err != nil {
			klog.Errorf("[cicd] failed to count running runs for pipeline %d: %v", rec.PipelineId, err)
			continue
		}
		if running >= c.maxRunningPerPipeline() {
			continue
		}
		now := time.Now()
		claimed, err := c.factory.CICD().ClaimRun(ctx, rec.Id, map[string]interface{}{
			"status":     model.CICDRunRunning,
			"started_at": now,
		})
		if err != nil {
			klog.Errorf("[cicd] failed to claim run %d: %v", rec.Id, err)
			continue
		}
		if !claimed {
			continue // 被其他 worker 抢先领取
		}
		c.executeRun(ctx, rec)
	}
}

// executeRun 创建 Tekton PipelineRun 并采集日志。
func (c *controller) executeRun(ctx context.Context, run *model.CICDRun) {
	pipeline, err := c.factory.CICD().GetPipeline(ctx, run.PipelineId)
	if err != nil || pipeline == nil {
		c.finishRunFailed(run.Id, "关联流水线不存在")
		return
	}
	engine, err := c.engine(ctx, run.ClusterId)
	if err != nil {
		c.finishRunFailed(run.Id, "构造引擎客户端失败: "+err.Error())
		return
	}

	var cfg struct {
		Repo struct {
			Ref        string `json:"ref"`
			Credential string `json:"credential"`
		} `json:"repo"`
	}
	if pipeline.Config != "" {
		_ = jsonUnmarshal([]byte(pipeline.Config), &cfg)
	}
	ref := run.GitRef
	if ref == "" {
		ref = cfg.Repo.Ref
	}
	if ref == "" {
		ref = "main"
	}

	spec := RunSpec{
		EngineName:      run.EngineName,
		Namespace:       run.Namespace,
		Config:          pipelineConfig(pipeline.Config),
		PipelineYaml:    pipeline.PipelineYaml,
		SpecMode:        pipeline.SpecMode,
		Template:        pipeline.Template,
		TimeoutSeconds:  pipeline.TimeoutSeconds,
		CredentialLabel: cfg.Repo.Credential,
	}

	sink := newCICDLogSink(c.cicdLogPath(run.Id))
	defer sink.Close()
	sink.Printf("开始执行流水线 %s (run %d), ref=%s", pipeline.Name, run.Id, ref)

	if _, err = engine.CreateRun(ctx, spec); err != nil {
		sink.Printf("创建 PipelineRun 失败: %v", err)
		c.finishRunFailed(run.Id, "创建 PipelineRun 失败: "+err.Error())
		return
	}
	sink.Printf("PipelineRun %s 已创建，等待执行完成", run.EngineName)

	// 跟随状态直到终态（ctx 取消或超时退出）。
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	timeout := time.Duration(pipeline.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 20 * time.Minute
	}
	deadline := time.Now().Add(timeout)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		// 采集日志（一次性拉取当前 Pod 日志追加到文件）。
		c.collectRunLogs(ctx, engine, run, sink)
		status, err := engine.GetRun(ctx, run.Namespace, run.EngineName)
		if err != nil {
			sink.Printf("查询 PipelineRun 状态失败: %v", err)
			continue
		}
		if status.Finished {
			c.applyRunStatus(run.Id, status, sink)
			return
		}
		if time.Now().After(deadline) {
			_ = engine.Cancel(ctx, run.Namespace, run.EngineName)
			c.finishRunStatus(run.Id, model.CICDRunTimeout, "运行超时", "", sink)
			return
		}
	}
}

// collectRunLogs 拉取各 TaskRun Pod 日志追加写入 per-run 文件。
func (c *controller) collectRunLogs(ctx context.Context, engine Engine, run *model.CICDRun, sink *cicdLogSink) {
	pods, err := engine.ListPods(ctx, run.Namespace, run.EngineName)
	if err != nil {
		return
	}
	for _, pod := range pods {
		key := fmt.Sprintf("%d/%s", run.Id, pod.Name)
		if sink.seenPod(key) {
			continue
		}
		sink.markPod(key)
		client, err := c.podLogClient(ctx, run.ClusterId)
		if err != nil {
			continue
		}
		container := "step-" + pod.Step
		req := client.Pods(run.Namespace).GetLogs(pod.Name, &corev1api.PodLogOptions{Container: container})
		stream, err := req.Stream(ctx)
		if err != nil {
			// 容器名可能不同，回退到不带 container。
			req = client.Pods(run.Namespace).GetLogs(pod.Name, &corev1api.PodLogOptions{})
			stream, err = req.Stream(ctx)
			if err != nil {
				continue
			}
		}
		sink.Printf("--- 步骤 [%s] 日志 ---", pod.Step)
		scanner := bufio.NewScanner(stream)
		for scanner.Scan() {
			sink.Printf("%s", scanner.Text())
		}
		_ = stream.Close()
	}
}

// syncRunStatus 单条 run 状态回查（启动对账 / 手动触发均可复用）。
func (c *controller) syncRunStatus(ctx context.Context, run *model.CICDRun) {
	engine, err := c.engine(ctx, run.ClusterId)
	if err != nil {
		return
	}
	status, err := engine.GetRun(ctx, run.Namespace, run.EngineName)
	if err != nil {
		return
	}
	if status.Finished {
		c.applyRunStatus(run.Id, status, nil)
	}
}

func (c *controller) applyRunStatus(runId int64, status *RunStatus, sink *cicdLogSink) {
	digest := status.Digest
	c.finishRunStatus(runId, status.Status, status.Message, digest, sink)
}

func (c *controller) finishRunFailed(runId int64, message string) {
	c.finishRunStatus(runId, model.CICDRunFailed, message, "", nil)
}

func (c *controller) finishRunStatus(runId int64, status model.CICDRunStatus, message, digest string, sink *cicdLogSink) {
	now := time.Now()
	updates := map[string]interface{}{
		"status":      status,
		"message":     message,
		"finished_at": now,
	}
	if digest != "" {
		updates["image_digest"] = digest
	}
	// duration 由 started_at 推算。
	if run, err := c.factory.CICD().GetRun(context.Background(), runId); err == nil && run != nil && run.StartedAt != nil {
		updates["duration_ms"] = now.Sub(*run.StartedAt).Milliseconds()
	}
	if err := c.factory.CICD().UpdateRun(context.Background(), runId, updates); err != nil {
		klog.Errorf("[cicd] failed to finalize run %d: %v", runId, err)
	}
	if sink != nil {
		sink.Printf("运行结束，状态=%s", status)
	}
}

// ---- 日志落盘与 SSE ----

type cicdLogSink struct {
	mu    sync.Mutex
	f     *os.File
	pods  map[string]bool
}

func newCICDLogSink(path string) *cicdLogSink {
	s := &cicdLogSink{pods: map[string]bool{}}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return s
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return s
	}
	s.f = f
	return s
}

func (s *cicdLogSink) Printf(format string, args ...interface{}) {
	if s == nil || s.f == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Fprintf(s.f, "[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), fmt.Sprintf(format, args...))
}

func (s *cicdLogSink) seenPod(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pods[key]
}

func (s *cicdLogSink) markPod(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pods[key] = true
}

func (s *cicdLogSink) Close() {
	if s == nil || s.f == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.f.Close()
}

func (c *controller) cicdLogPath(runId int64) string {
	dir := c.cc.CICD.LogDir
	if dir == "" {
		dir = "/tmp/pixiu/cicd-logs"
	}
	return filepath.Join(dir, fmt.Sprintf("run-%d.log", runId))
}

// WatchRunLogs SSE：先回放文件再 follow，客户端断开即退出。
func (c *controller) WatchRunLogs(ctx context.Context, id int64, w http.ResponseWriter, r *http.Request) error {
	if _, err := c.run(ctx, id); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flush, _ := w.(http.Flusher)

	path := c.cicdLogPath(id)
	var offset int64
	for {
		select {
		case <-r.Context().Done():
			return nil
		case <-ctx.Done():
			return nil
		default:
		}
		next, err := tailFile(w, path, offset)
		if err != nil {
			return nil
		}
		if next != offset {
			offset = next
			if flush != nil {
				flush.Flush()
			}
		}
		time.Sleep(logPollInterval)
	}
}

// tailFile 从 offset 读取文件新增内容并以 SSE data 帧写出，返回新的 offset。
func tailFile(w http.ResponseWriter, path string, offset int64) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return offset, nil
		}
		return offset, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return offset, err
	}
	if info.Size() <= offset {
		return offset, nil
	}
	if _, err = f.Seek(offset, io.SeekStart); err != nil {
		return offset, err
	}
	reader := bufio.NewReader(f)
	buf := make([]byte, 4096)
	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			for _, line := range strings.Split(string(buf[:n]), "\n") {
				if line != "" {
					fmt.Fprintf(w, "data: %s\n\n", line)
				}
			}
			offset += int64(n)
		}
		if readErr != nil {
			break
		}
	}
	return offset, nil
}

// ---- pod log client ----

func (c *controller) podLogClient(ctx context.Context, clusterId int64) (corev1.PodsGetter, error) {
	cluster, err := c.factory.Cluster().Get(ctx, clusterId)
	if err != nil {
		return nil, err
	}
	if cluster == nil {
		return nil, fmt.Errorf("cluster %d not found", clusterId)
	}
	clusterSet, err := c.clusters.GetClusterSetByName(ctx, cluster.Name)
	if err != nil {
		return nil, err
	}
	cs, err := kubernetes.NewForConfig(clusterSet.Config)
	if err != nil {
		return nil, err
	}
	return cs.CoreV1(), nil
}
