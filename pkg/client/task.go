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
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

type WrapObject struct {
	LatestTime time.Time    // 最近一次获取时间
	Object     []model.Task // TODO，临时实现，使用task的结构，后续优化成任意类型
}

type Task struct {
	sync.RWMutex

	Lister func(ctx context.Context, planId int64, opts ...db.Options) ([]model.Task, error)
	items  map[int64]WrapObject
}

func NewTaskCache() *Task {
	t := &Task{items: make(map[int64]WrapObject)}
	t.Run()

	return t
}

func (t *Task) SetLister(Lister func(ctx context.Context, planId int64, opts ...db.Options) ([]model.Task, error)) {
	t.Lock()
	defer t.Unlock()

	t.Lister = Lister
}

func (t *Task) Get(planId int64) ([]model.Task, bool) {
	t.Lock()
	defer t.Unlock()

	wrapObject, ok := t.items[planId]
	if !ok {
		return nil, false
	}

	wrapObject.LatestTime = time.Now()
	t.items[planId] = wrapObject
	return wrapObject.Object, ok
}

func (t *Task) Set(planId int64, tasks []model.Task) {
	t.Lock()
	defer t.Unlock()

	if t.items == nil {
		t.items = map[int64]WrapObject{}
	}

	now := time.Now()
	klog.Infof("add plan(%d) tasks into cache at %v", planId, now)
	t.items[planId] = WrapObject{
		Object:     tasks,
		LatestTime: now,
	}
}

func (t *Task) SetByTask(planId int64, task model.Task) {
	t.Lock()
	defer t.Unlock()

	wrapObject, ok := t.items[planId]
	if !ok {
		return
	}

	var (
		index int
		found bool
	)
	for i, s := range wrapObject.Object {
		if s.Id == task.Id {
			index = i
			found = true
			break
		}
	}
	if !found {
		return
	}

	wrapObject.Object[index] = task
	wrapObject.LatestTime = time.Now()

	t.items[planId] = wrapObject
}

func (t *Task) Delete(planId int64) {
	t.Lock()
	defer t.Unlock()

	delete(t.items, planId)
}

// WaitForCacheSync
// 判断缓存中是否已经存在，如果不存在则先写入
func (t *Task) WaitForCacheSync(planId int64) error {
	_, ok := t.Get(planId)
	if ok {
		return nil
	}

	tasks, err := t.Lister(context.TODO(), planId)
	if err != nil {
		return fmt.Errorf("failed to get plan(%d) tasks from database: %v", planId, err)
	}
	t.Set(planId, tasks)

	return nil
}

func (t *Task) syncTasks() {
	if t.Lister == nil || len(t.items) == 0 {
		klog.V(2).Infof("syncing and waiting for the next loop")
		return
	}

	t.Lock()
	defer t.Unlock()

	for planId, wrapObject := range t.items {
		now := time.Now()
		if now.Sub(wrapObject.LatestTime) > 5*time.Second {
			// 如果对象5分钟未被操作，则从缓存中清理
			klog.Infof("remove plan(%d) tasks from cache due to it not be handled for 5s", planId)
			delete(t.items, planId)
			// 处理下一个对象
			continue
		}

		newTasks, err := t.Lister(context.TODO(), planId)
		if err != nil {
			klog.Errorf("[syncTasks] failed to list plan(%d) tasks: %v", planId, err)
			delete(t.items, planId)
			continue
		}
		t.items[planId] = WrapObject{
			Object:     newTasks,
			LatestTime: now,
		}
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
