/*
Copyright 2021 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (phe "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package plan

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/caoyingjunz/pixiu/pkg/util/runtime"
)

// TaskInterface 计划任务子接口
type TaskInterface interface {
	List(ctx context.Context, planId int64) ([]types.PlanTask, error)
	// Watch SSE 实时推送任务状态
	Watch(ctx context.Context, planId int64, w http.ResponseWriter, r *http.Request)
	// WatchLog SSE 实时推送任务容器日志
	WatchLog(ctx context.Context, planId int64, taskId int64, w http.ResponseWriter, r *http.Request) error
}

type planTask struct {
	p *plan
}

func (p *plan) Task() TaskInterface {
	return &planTask{p: p}
}

func (t *planTask) List(ctx context.Context, planId int64) ([]types.PlanTask, error) {
	objects, err := t.p.factory.Plan().Task().List(ctx, planId)
	if err != nil {
		klog.Errorf("failed to get plan(%d) tasks: %v", planId, err)
		return nil, err
	}

	var tasks []types.PlanTask
	for _, object := range objects {
		tasks = append(tasks, *t.p.modelTask2Type(&object))
	}

	return tasks, nil
}

func (t *planTask) Watch(ctx context.Context, planId int64, w http.ResponseWriter, r *http.Request) {
	flush, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// 初始化 Lister
	if taskC.Lister == nil {
		taskC.SetLister(t.p.factory.Plan().Task().List)
	}
	// 等待缓存同步
	if err := taskC.WaitForCacheSync(planId); err != nil {
		return
	}

	for {
		select {
		case <-r.Context().Done():
			klog.Infof("plan(%d) watch API has been closed by client and cache will be removed after 5s", planId)
			return
		default:
			tasks, ok := taskC.Get(planId)
			if ok {
				var ts []types.PlanTask
				for _, object := range tasks {
					ts = append(ts, *t.p.modelTask2Type(&object))
				}
				if err := json.NewEncoder(w).Encode(ts); err != nil {
					klog.Errorf("failed to encode tasks: %v", err)
					break
				}
				flush.Flush()
			}

			// 同步事件间隔为 3s
			time.Sleep(3 * time.Second)
		}
	}
}

func (t *planTask) WatchLog(ctx context.Context, planId int64, taskId int64, w http.ResponseWriter, r *http.Request) error {
	task, err := t.p.factory.Plan().Task().GetByID(ctx, taskId)
	if err != nil {
		klog.Errorf("failed to get tasks of plan %d: %v", planId, err)
		return err
	}
	if task.Status == model.UnStartPlanStatus {
		return fmt.Errorf("任务尚未开始")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	rt, err := runtime.Default()
	if err != nil {
		return err
	}
	readCloser, err := rt.Logs(ctx, fmt.Sprintf("%s-%d", task.Action, planId), true)
	if err != nil {
		return err
	}
	defer readCloser.Close()

	// 解复用已归运行时实现（docker.go 内部用 StdCopy 解复用；containerd 读纯文本落盘日志），
	// 调用点二次解复用会得到空流，此为 #1061 panic 修复在抽象层的等价承接。
	scanner := bufio.NewScanner(readCloser)
	flush, _ := w.(http.Flusher)
	for scanner.Scan() {
		line := append(scanner.Bytes(), '\n')
		if _, err = w.Write(line); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		flush.Flush()
	}
	if err = scanner.Err(); err != nil && err != io.EOF {
		return err
	}

	return nil
}

func (p *plan) modelTask2Type(o *model.Task) *types.PlanTask {
	return &types.PlanTask{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
		Name:    o.Name,
		PlanId:  o.PlanId,
		Action:  o.Action,
		Status:  o.Status,
		Message: o.Message,
	}
}
