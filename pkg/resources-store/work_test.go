package resourcesstore

import (
	"context"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestWorker(t *testing.T) {
	config, _ := NewConfig()
	ctx := context.Background()

	rg := NewResourceGetter(ctx, config)
	Worker(rg)
	// fmt.Println(rg)
	// fmt.Println(StoreObj)

	gvr1 := schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}
	gvr2 := schema.GroupVersionResource{
		Version:  "v1",
		Resource: "services",
	}
	v1, _ := StoreObj.GetByNamespaceAndName(gvr1, "kube-system", "coredns")
	fmt.Println(v1)
	v2, _ := StoreObj.GetByNamespaceAndName(gvr2, "kube-system", "kube-dns")
	fmt.Println(v2)
	v3 := StoreObj.ListByNamespace(gvr2, "default")
	fmt.Println(v3)
	v4 := StoreObj.ListAll(gvr1)
	fmt.Println(v4)
}
