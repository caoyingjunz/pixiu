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

package deployagent

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/caoyingjunz/pixiu/pkg/util/runtime"
)

// agentRunWaitTimeout agent 侧等待 runner 容器的安全兜底上限：
// 取服务端最大单步预算（AgentJob.timeout 60min）的 2 倍，正常部署不可能触达；
// 仅当容器挂死时用于自恢复（containerd 会到点强杀，docker 仅结束等待，容器留存待下次同名任务回收）。
const agentRunWaitTimeout = 2 * time.Hour

func pullImage(ctx context.Context, ag *Agent, job *types.Job) error {
	klog.Infof("job %d: pulling image %s", job.Id, job.Image)

	_ = ag.Logs(job.Id, fmt.Sprintf("pulling image %s\n", job.Image))
	if err := ag.Runtime.PullImage(ctx, job.Image); err != nil {
		klog.Errorf("job %d: image pull failed: %v", job.Id, err)
		return err
	}

	klog.Infof("job %d: image %s pulled successfully", job.Id, job.Image)
	return ag.Report(job.Id, true, "image ready", "")
}

func runContainer(ctx context.Context, ag *Agent, workRoot string, job *types.Job) error {
	name := fmt.Sprintf("%s-%d", job.Action, job.PlanId)
	planDir := filepath.Join(workRoot, fmt.Sprintf("%d", job.PlanId))

	// 设置 WaitTimeout 为安全兜底（见 agentRunWaitTimeout，远大于服务端单步预算，兜底自恢复），
	// 正常部署由服务端步骤预算先行决定成败
	err := ag.Runtime.RunContainer(ctx, &runtime.ContainerSpec{
		Name:        name,
		Image:       job.Image,
		Env:         []string{fmt.Sprintf("COMMAND=%s", job.Action)},
		Binds:       []string{fmt.Sprintf("%s:/configs", planDir)},
		NetworkHost: true,
		WaitTimeout: agentRunWaitTimeout,
	})

	// 无论成功失败，都把容器日志上报服务端；容器不存在等场景下读取失败会被忽略
	if logs, logErr := ag.Runtime.Logs(ctx, name, false); logErr == nil {
		b, _ := io.ReadAll(logs)
		_ = logs.Close()
		_ = ag.Logs(job.Id, string(b))
	}
	if err != nil {
		return err
	}

	return ag.Report(job.Id, true, "ok", "")
}
