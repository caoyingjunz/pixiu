/*
Copyright 2021 The Pixiu Authors.
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

package client

import (
	"context"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

type Task struct {
	sync.RWMutex

	Lister func(ctx context.Context, planId int64) ([]model.Task, error)
	items  map[int64][]model.Task
}

func NewTaskCache() *Task {
	t := &Task{items: make(map[int64][]model.Task)}
	t.Run()

	return t
}

func (t *Task) SetLister(Lister func(ctx context.Context, planId int64) ([]model.Task, error)) {
	t.Lock()
	defer t.Unlock()

	t.Lister = Lister
}

func (t *Task) Get(planId int64) ([]model.Task, bool) {
	t.Lock()
	defer t.Unlock()

	tasks, ok := t.items[planId]
	return tasks, ok
}

func (t *Task) Set(planId int64, tasks []model.Task) {
	t.Lock()
	defer t.Unlock()

	if t.items == nil {
		t.items = map[int64][]model.Task{}
	}
	t.items[planId] = tasks
}

func (t *Task) Delete(planId int64) {
	t.Lock()
	defer t.Unlock()

	delete(t.items, planId)
}

func (t *Task) syncTasks() {
	if t.Lister == nil || len(t.items) == 0 {
		klog.Infof("syncing and waiting for the next loop")
		return
	}

	t.Lock()
	defer t.Unlock()

	for planId := range t.items {
		newTasks, err := t.Lister(context.TODO(), planId)
		if err != nil {
			klog.Errorf("[syncTasks] failed to list plan(%d) tasks: %v", planId, err)
			delete(t.items, planId)
			continue
		}
		t.items[planId] = newTasks
	}
}

// Run 启动 syncTasks
// TODO: 后续优化 stopCh
func (t *Task) Run() {
	stopCh := make(<-chan struct{})
	// 10s 主动加载一次 task
	// 不发起长连接情况下，无任何开销
	go wait.Until(t.syncTasks, 10*time.Second, stopCh)
}
