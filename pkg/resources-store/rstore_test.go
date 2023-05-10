package resourcesstore

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
)

func TestScoop(t *testing.T) {
	ctx := context.Background()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}

	clientset, _ := NewClientSet()
	informer, _ := NewInformerForResource(ctx, clientset, gvr)
	// itype := reflect.TypeOf(informer)
	// fmt.Println(itype)
	objs, _ := ListResources(informer)
	// for _, obj := range objs {
	// 	fmt.Printf("%+v\n", obj)
	// 	fmt.Println()
	// }

	// fmt.Printf("%+v\n", objs)
	// fmt.Println("==================")

	fmt.Println()
	byteData, _ := json.Marshal(objs[0])
	fmt.Println(string(byteData))

	itype := reflect.TypeOf(objs).Elem()
	fmt.Println(itype)
	itype3, ok := scheme.Scheme.AllKnownTypes()[gvk]
	fmt.Println(itype3)
	if ok {
		fmt.Println(".........")
	}
	switch itype {
	case itype3:
		pod := objs[0].(*corev1.Pod)
		// fmt.Printf("%+v\n", pod)
		// fmt.Println("=========")
		// fmt.Println(pod.ObjectMeta)
		byteData, _ := json.Marshal(pod)
		fmt.Println(string(byteData))
	}
}

func TestGetPod(t *testing.T) {
	ctx := context.Background()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}

	clientset, _ := NewClientSet()
	informer, _ := NewInformerForResource(ctx, clientset, gvr)
	objs, _ := ListResources(informer)
	byteData, _ := json.Marshal(objs)
	fmt.Println(string(byteData))
}
