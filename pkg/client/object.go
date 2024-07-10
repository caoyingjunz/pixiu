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
)

type Object interface {
	LastSeenTime() time.Time
	LastUpdateTime() time.Time
	Object() interface{}
}

type item struct {
	sync.RWMutex

	LastSeenTime   time.Time
	LastUpdateTime time.Time
	Object         interface{}
}

type object struct {
	sync.RWMutex

	Lister func(ctx context.Context) (interface{}, error)
	items  map[interface{}]item
}

func NewObject() *object {
	return &object{items: make(map[interface{}]item)}
}

func (o *object) SetLister(Lister func(ctx context.Context) (interface{}, error)) {
	o.Lock()
	defer o.Unlock()

	o.Lister = Lister
}

func (o *object) Get(key interface{}) (interface{}, bool) {
	o.Lock()
	defer o.Unlock()

	obj, ok := o.items[key]
	if !ok {
		return nil, false
	}

	obj.LastSeenTime = time.Now()
	o.items[key] = obj
	return obj.Object, ok
}

func (o *object) Set(key interface{}, object interface{}) {
	o.Lock()
	defer o.Unlock()

	o.items[key] = item{
		Object:         object,
		LastUpdateTime: time.Now(),
	}
}

func (o *object) Delete(key interface{}) {
	o.Lock()
	defer o.Unlock()

	delete(o.items, key)
}

// 通过 lister 自动同步最新数据
func (o *object) sync() {
	if o.Lister == nil || len(o.items) == 0 {
		klog.V(2).Infof("syncing and waiting for the next sync")
		return
	}

	o.Lock()
	defer o.Unlock()

	for k, v := range o.items {
		now := time.Now()
		if now.Sub(v.LastSeenTime) > 5*time.Second {
			// 如果对象已经超时，则从缓存中清理
			delete(o.items, k)
			continue
		}

		// TODO: 优化
		newObject, err := o.Lister(context.TODO())
		if err != nil {
			delete(o.items, k)
			continue
		}
		o.items[k] = item{
			Object:         newObject,
			LastSeenTime:   now,
			LastUpdateTime: now,
		}
	}
}

// Run 启动 syncTasks
// TODO: 后续优化 stopCh
func (o *object) Run() {
	stopCh := make(<-chan struct{})
	// 10s 主动加载一次 task
	// 不发起长连接情况下，无任何开销
	go wait.Until(o.sync, 10*time.Second, stopCh)
}
