package resourcesstore

import (
	"context"
	"fmt"
	"testing"
)

// func TestScoop(t *testing.T) {
// 	ctx := context.Background()
// 	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
// 	gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}

// 	clientset, _ := NewClientSet()
// 	informer, _ := NewInformerForResource(ctx, clientset, gvr)
// 	// itype := reflect.TypeOf(informer)
// 	// fmt.Println(itype)
// 	objs, _ := ListResources(informer)
// 	// for _, obj := range objs {
// 	// 	fmt.Printf("%+v\n", obj)
// 	// 	fmt.Println()
// 	// }

// 	// fmt.Printf("%+v\n", objs)
// 	// fmt.Println("==================")

// 	fmt.Println()
// 	byteData, _ := json.Marshal(objs[0])
// 	fmt.Println(string(byteData))

// 	itype := reflect.TypeOf(objs).Elem()
// 	fmt.Println(itype)
// 	itype3, ok := scheme.Scheme.AllKnownTypes()[gvk]
// 	fmt.Println(itype3)
// 	if ok {
// 		fmt.Println(".........")
// 	}
// 	switch itype {
// 	case itype3:
// 		pod := objs[0].(*corev1.Pod)
// 		// fmt.Printf("%+v\n", pod)
// 		// fmt.Println("=========")
// 		// fmt.Println(pod.ObjectMeta)
// 		byteData, _ := json.Marshal(pod)
// 		fmt.Println(string(byteData))
// 	}
// }

// func TestGetPod(t *testing.T) {
// 	ctx := context.Background()
// 	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}

// 	clientset, _ := NewClientSet()
// 	informer, _ := NewInformerForResource(ctx, clientset, gvr)
// 	objs, _ := ListResources(informer)
// 	// byteData, _ := json.Marshal(objs)
// 	// fmt.Println(string(byteData))

// 	obj := objs[0]
// 	str, _ := cache.MetaNamespaceKeyFunc(obj)
// 	fmt.Println(str)

// 	// gvk := obj.GetObjectKind().GroupVersionKind()
// 	// restmapper.
// }

// func TestGvkToGVR(t *testing.T) {
// 	gvk := schema.GroupVersionKind{
// 		Group:   "apps",
// 		Version: "v1",
// 		Kind:    "Deployment",
// 	}

// 	client, _ := NewDiscoveryClient()
// 	gvr, _ := GVKToGVR(client, gvk)
// 	fmt.Println(gvr.String())
// }

// func TestGetGVRs(t *testing.T) {
// 	client, _ := NewDiscoveryClient()
// 	gvrs, _ := GetGVRs(client)
// 	fmt.Printf("%v\n", gvrs)
// }

func TestResourceGetter(t *testing.T) {
	config, _ := NewConfig()
	ctx := context.Background()

	rg := NewResourceGetter(ctx, config)
	fmt.Println(rg)
}
