package resourcesstore

// 由于 github 上没有办法生成 kubeconfig ，test 会失败，test case 注释掉
// func TestWorker(t *testing.T) {
// 	config, _ := NewConfig()
// 	ctx := context.Background()

// 	rg := NewResourceGetter(ctx, config)
// 	Worker(rg)
// 	// fmt.Println(rg)
// 	// fmt.Println(rg.store)

// 	gvr1 := schema.GroupVersionResource{
// 		Group:    "apps",
// 		Version:  "v1",
// 		Resource: "deployments",
// 	}
// 	gvr2 := schema.GroupVersionResource{
// 		Version:  "v1",
// 		Resource: "services",
// 	}
// 	v1, _ := rg.store.GetByNamespaceAndName(gvr1, "kube-system", "coredns")
// 	fmt.Println(v1)
// 	v2, _ := rg.store.GetByNamespaceAndName(gvr2, "kube-system", "kube-dns")
// 	fmt.Println(v2)
// 	v3 := rg.store.ListByNamespace(gvr2, "default")
// 	fmt.Println(v3)
// 	v4 := rg.store.ListAll(gvr1)
// 	fmt.Println(v4)
// }

// func TestProcess(t *testing.T) {
// 	config, _ := NewConfig()
// 	ctx := context.Background()
// 	rg := NewResourceGetter(ctx, config)
// 	klog.Infof("ResourceGetter info: %+v\n", rg)

// 	go Process(rg)

// 	gvr1 := schema.GroupVersionResource{
// 		Group:    "apps",
// 		Version:  "v1",
// 		Resource: "deployments",
// 	}

// 	// 周期性获取资源
// 	ticker := time.NewTicker(1500 * time.Millisecond)
// 	defer ticker.Stop()

// 	for {
// 		select {
// 		case <-ticker.C:
// 			v1, _ := rg.store.GetByNamespaceAndName(gvr1, "kube-system", "coredns")
// 			fmt.Println(v1)
// 		case <-rg.ctx.Done():
// 			return
// 		}
// 	}
// }
