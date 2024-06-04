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
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"text/template"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/util"
	"github.com/caoyingjunz/pixiu/pkg/util/errors"
	pixiutpl "github.com/caoyingjunz/pixiu/template"
)

func (p *plan) Run(ctx context.Context, workers int) error {
	klog.Infof("Starting Plan Manager")
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, p.worker, time.Second)
	}
	return nil
}

func (p *plan) worker(ctx context.Context) {
	for p.process(ctx) {
	}
}

func (p *plan) process(ctx context.Context) bool {
	key, quit := taskQueue.Get()
	if quit {
		return false
	}
	defer taskQueue.Done(key)

	p.syncHandler(ctx, key.(int64))
	return true
}

type TaskData struct {
	PlanId int64
	Config *model.Config
	Nodes  []model.Node
}

func (t TaskData) validate() error {
	return nil
}

func (p *plan) getTaskData(ctx context.Context, planId int64) (TaskData, error) {
	nodes, err := p.factory.Plan().ListNodes(ctx, planId)
	if err != nil {
		return TaskData{}, err
	}
	cfg, err := p.factory.Plan().GetConfigByPlan(ctx, planId)
	if err != nil {
		return TaskData{}, err
	}

	return TaskData{
		PlanId: planId,
		Config: cfg,
		Nodes:  nodes,
	}, nil
}

// 实际处理函数
// 处理步骤:
// 1. 检查部署参数是否符合要求
// 2. 渲染环境
// 3. 执行部署
// 4. 部署后环境清理
func (p *plan) syncHandler(ctx context.Context, planId int64) {
	klog.Infof("starting plan(%d) task", planId)

	taskData, err := p.getTaskData(ctx, planId)
	if err != nil {
		klog.Errorf("failed to get task data: %v", err)
		return
	}

	task := newHandlerTask(taskData)
	handlers := []Handler{
		Check{handlerTask: task},
		Render{handlerTask: task},
		BootStrap{handlerTask: task},
	}
	if err = p.syncTasks(handlers...); err != nil {
		klog.Errorf("failed to sync task: %v", err)
	}
}

func (p *plan) syncTasks(tasks ...Handler) error {
	for _, task := range tasks {
		planId := task.GetPlanId()
		name := task.Name()

		var (
			object *model.Task
			err    error
		)
		object, err = p.factory.Plan().GetTaskByName(context.TODO(), planId, name)
		if err != nil {
			if !errors.IsRecordNotFound(err) {
				return err
			}

			object, err = p.factory.Plan().CreatTask(context.TODO(), &model.Task{
				Name:   name,
				PlanId: planId,
				Step:   model.RunningPlanStep,
			})
			if err != nil {
				klog.Errorf("failed to init plan(%d) task(%s): %v", object.PlanId, name, err)
				return err
			}
		}

		status := model.SuccessPlanStatus
		step := task.Step()
		message := ""

		// 执行检查
		if err = task.Run(); err != nil {
			status = model.FailedPlanStatus
			step = model.FailedPlanStep
			message = err.Error()
		}

		// 执行完成之后更新状态
		if err = p.factory.Plan().UpdateTask(context.TODO(), object.PlanId, object.ResourceVersion, map[string]interface{}{
			"status":  status,
			"message": message,
			"step":    step,
		}); err != nil {
			klog.Errorf("failed to update plan(%d) task(%s): %v", object.PlanId, name, err)
			return err
		}
	}

	return nil
}

type Handler interface {
	GetPlanId() int64

	Name() string         // 检查项名称
	Step() model.PlanStep // 未开始，运行中，异常和完成
	Run() error           // 执行
}

type handlerTask struct {
	data TaskData
}

func (t handlerTask) GetPlanId() int64     { return t.data.PlanId }
func (t handlerTask) Step() model.PlanStep { return model.RunningPlanStep }

func newHandlerTask(data TaskData) handlerTask {
	return handlerTask{data: data}
}

type Check struct {
	handlerTask
}

func (c Check) Name() string { return "部署预检查" }
func (c Check) Run() error {
	if err := c.data.validate(); err != nil {
		return err
	}
	return nil
}

// Render 渲染 pixiu 部署配置
// 1. 渲染 hosts
// 2. 渲染 globals.yaml
// 3. 渲染 multinode
// 具体参考 https://github.com/pixiu-io/kubez-ansible
type Render struct {
	handlerTask
}

func (r Render) Name() string { return "配置渲染" }
func (r Render) Run() error {
	// 渲染 hosts
	if err := r.doRender("hosts", pixiutpl.HostTemplate, r.data); err != nil {
		return err
	}
	// 渲染 multiNode
	if err := r.doRender("multinode", pixiutpl.MultiModeTemplate, ParseMultinode(r.data)); err != nil {
		return err
	}
	// 渲染 globals
	if err := r.doRender("globals.yml", pixiutpl.GlobalsTemplate, ParseGlobal(r.data)); err != nil {
		return err
	}

	return nil
}

func (r Render) doRender(name string, text string, data interface{}) error {
	tpl := template.New(name)
	tpl = template.Must(tpl.Parse(text))

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return err
	}
	filename, err := getFileForRender(r.GetPlanId(), name)
	if err != nil {
		return err
	}
	if err = WriteToFile(filename, buf.Bytes()); err != nil {
		return err
	}

	return nil
}

func WriteToFile(filename string, data []byte) error {
	return ioutil.WriteFile(filename, data, 0644)
}

const (
	workDir = "/tmp"
)

// 后续优化
func getFileForRender(planId int64, f string) (string, error) {
	planDir := filepath.Join(workDir, fmt.Sprintf("%d", planId))
	if err := util.EnsureDirectoryExists(planDir); err != nil {
		return "", err
	}

	return filepath.Join(planDir, f), nil
}

type Multinode struct {
	DockerMaster     []string
	DockerNode       []string
	ContainerdMaster []string
	ContainerdNode   []string
}

func ParseMultinode(data TaskData) Multinode {
	multinode := Multinode{
		DockerMaster:     make([]string, 0),
		DockerNode:       make([]string, 0),
		ContainerdMaster: make([]string, 0),
		ContainerdNode:   make([]string, 0),
	}
	for _, node := range data.Nodes {
		if node.Role == model.MasterRole {
			multinode.DockerMaster = append(multinode.DockerMaster, node.Name)
		}
		if node.Role == model.NodeRole {
			multinode.DockerNode = append(multinode.DockerNode, node.Name)
		}
	}

	return multinode
}

type Global struct {
}

func ParseGlobal(data TaskData) Global {
	return Global{}
}
