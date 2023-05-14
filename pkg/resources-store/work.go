package resourcesstore

import (
	"context"
	"time"

	"k8s.io/klog/v2"
)

// 运行整个模块
func Worker(rg *resourceGetter) {
	rg.sharedInformerFactory.Start(rg.ctx.Done())
	rg.sharedInformerFactory.WaitForCacheSync(rg.ctx.Done())

	// 先看一个gvr的获取--测试 pass
	// gvr := schema.GroupVersionResource{
	// 	Group:    "apps",
	// 	Version:  "v1",
	// 	Resource: "deployments",
	// }
	// informer := rg.informers[gvr]
	// objs, _ := rg.ListResources(informer)
	// klog.Infof("objs: %+v\n", objs)

	for gvr, informer := range rg.informers {
		objs, err := rg.ListResources(informer)
		if err != nil {
			klog.Warningf("list resource failed, gvr: %+v, err: %v", gvr, err)
			continue
		}

		// TODO: 后续优化 store 的逻辑
		StoreObj.Add(gvr, objs)
	}
	// flag 处理
	if StoreObj.flag == writeM {
		StoreObj.flag = writeMbk
	} else {
		StoreObj.flag = writeM
	}
}

func Process() {
	config, _ := NewConfig()
	ctx := context.Background()

	rg := NewResourceGetter(ctx, config)
	klog.Infof("ResourceGetter info: %+v\n", rg)

	go func() {
		ticker := time.NewTicker(rg.period)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				Worker(rg)
			case <-ctx.Done():
				return
			}
		}
	}()

	<-ctx.Done()
}
