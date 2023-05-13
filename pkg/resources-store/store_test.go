package resourcesstore

import (
	"context"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// func TestStore(t *testing.T) {
// 	// 获取 pod 资源
// 	ctx := context.Background()
// 	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
// 	// fmt.Println(gvr.String())
// 	store := NewStore()

// 	clientset, _ := NewClientSet()
// 	informer, _ := NewInformerForResource(ctx, clientset, gvr)
// 	objs, _ := ListResources(informer)

// 	store.Add(gvr, objs)

// 	fmt.Println()
// 	fmt.Println("=============")
// 	key := storeKeyFunc(gvr)
// 	fmt.Printf("%+v\n", store.m[key])
// 	// entry := newEntry()
// 	// entry.add(objs)
// 	// fmt.Println(entry)
// }

// func TestList(t *testing.T) {
// 	// 获取 pod 资源
// 	ctx := context.Background()
// 	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
// 	// fmt.Println(gvr.String())
// 	store := NewStore()

// 	clientset, _ := NewClientSet()
// 	informer, _ := NewInformerForResource(ctx, clientset, gvr)
// 	objs, _ := ListResources(informer)

// 	store.Add(gvr, objs)

// 	v, _ := store.Get(gvr, "default", "nginx-deployment-86dd747df5-9rtms")
// 	fmt.Println(v)
// }

func TestListByNamespace(t *testing.T) {
	config, _ := NewConfig()
	ctx := context.Background()

	rg := NewResourceGetter(ctx, config)
	Worker(rg)

	gvr1 := schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}

	v1 := StoreObj.ListByNamespace(gvr1, "kube-system")
	fmt.Println(v1)
	fmt.Println(len(v1))
}
