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

package resourcesstore

import (
	"time"

	"k8s.io/klog/v2"
)

// 运行整个模块
func Worker(rg *resourceGetter) {
	rg.sharedInformerFactory.Start(rg.ctx.Done())
	rg.sharedInformerFactory.WaitForCacheSync(rg.ctx.Done())

	for gvr, informer := range rg.informers {
		objs, err := rg.ListResources(informer)
		if err != nil {
			klog.Warningf("list resource failed, gvr: %+v, err: %v", gvr, err)
			continue
		}

		rg.store.Add(gvr, objs)
	}
	// flag 处理
	rg.store.PostAdd()
}

func Process(rg *resourceGetter) {
	go func() {
		ticker := time.NewTicker(rg.period)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				Worker(rg)
			case <-rg.ctx.Done():
				return
			}
		}
	}()

	<-rg.ctx.Done()
}
