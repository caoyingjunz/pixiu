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
	"github.com/caoyingjunz/pixiu/pkg/util/container"
)

// RunTask
// 只运行指定的计划任务
func (p *plan) RunTask(ctx context.Context, planId int64, taskId int64) error {
	return nil
}

func (p *plan) ListTasks(ctx context.Context, planId int64) ([]types.PlanTask, error) {
	objects, err := p.factory.Plan().ListTasks(ctx, planId)
	if err != nil {
		klog.Errorf("failed to get plan(%d) tasks: %v", planId, err)
		return nil, err
	}

	var tasks []types.PlanTask
	for _, object := range objects {
		tasks = append(tasks, *p.modelTask2Type(&object))
	}

	return tasks, nil
}

func (p *plan) WatchTasks(ctx context.Context, planId int64, w http.ResponseWriter, r *http.Request) {
	flush, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// 初始化 Lister
	if taskC.Lister == nil {
		taskC.SetLister(p.factory.Plan().ListTasks)
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
					ts = append(ts, *p.modelTask2Type(&object))
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

func (p *plan) WatchTaskLog(ctx context.Context, planId int64, taskId int64, w http.ResponseWriter, r *http.Request) error {
	task, err := p.factory.Plan().GetTaskById(ctx, taskId)
	if err != nil {
		klog.Errorf("failed to get tasks of plan %d: %v", planId, err)
		return err
	}

	if task.Status == model.UnStartPlanStatus {
		return fmt.Errorf("任务尚未开始")
	}

	c, err := container.NewContainer("WatchTaskLog", planId, "")
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// TODO 临时指定，后期根据步骤id去做查询判断
	var step string
	switch task.Name {
	case "初始化部署环境":
		step = "bootstrap-servers"
	case "部署Master":
		step = "deploy"
	case "部署Node":
		step = "deploy"
	case "部署基础组件":
		step = "deploy"
	default:
		step = "bootstrap-servers"
	}

	containerId := fmt.Sprintf("%s-%d", step, planId)
	readCloser, err := c.WatchContainerLog(ctx, containerId, "")
	if err != nil {
		return err
	}
	defer readCloser.Close()

	// 读取日志
	scanner := bufio.NewScanner(readCloser)
	flush, _ := w.(http.Flusher)
	for scanner.Scan() {
		line := append(scanner.Bytes(), byte('\n'))
		// 去掉前8不可见字符
		_, err = w.Write(line[8:])
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		flush.Flush()
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
		Status:  o.Status,
		Message: o.Message,
	}
}

func (p *plan) modelTask2TypeList(o []*model.Task) []types.PlanTask {
	var tasks []types.PlanTask
	for _, object := range o {
		tasks = append(tasks, *p.modelTask2Type(object))
	}
	return tasks
}
