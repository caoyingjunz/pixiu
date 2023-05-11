package resourcesstore

import (
	"context"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

// 首先根据 config 的路径生成 client
func NewClientSet() (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		klog.Errorf("build config from ~/.kube/config failed, err: ", err)
		klog.Info("try to build config from cluster.")
		inClusterConfig, err := rest.InClusterConfig()
		if err != nil {
			klog.Fatalf("build config from cluster failed, err: ", err)
			return nil, err
		}
		config = inClusterConfig
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("build clientSet failed, err: ", err)
		return nil, err
	}

	return clientSet, err
}

// 对某种资源生成 informer
func NewInformerForResource(ctx context.Context, clientSet *kubernetes.Clientset, resource schema.GroupVersionResource) (informers.GenericInformer, error) {
	informerFactory := informers.NewSharedInformerFactory(clientSet, 0)
	informer, err := informerFactory.ForResource(resource)
	if err != nil {
		klog.Errorf("create informer for resource: %+v failed", resource)
		return nil, err
	}

	informerFactory.Start(ctx.Done())
	informerFactory.WaitForCacheSync(ctx.Done())

	return informer, nil
}

// 获取 resource
func ListResources(informer informers.GenericInformer) ([]runtime.Object, error) {
	lister := informer.Lister()
	objs, err := lister.List(labels.Everything())
	if err != nil {
		klog.Errorf("list resource failed")
		return nil, err
	}

	return objs, nil
}

// TODO: 根据 http 请求解析 gvr
func ParseHttp() (schema.GroupVersionResource, error) {
	var resource schema.GroupVersionResource
	return resource, nil
}

//
