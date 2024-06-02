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
	"context"
	"fmt"
	"time"

	"k8s.io/klog/v2"

	"k8s.io/apimachinery/pkg/util/wait"
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

// 实际处理函数
func (p *plan) syncHandler(ctx context.Context, planId int64) {
	fmt.Println("start planId", planId)
}
