package resourcesstore

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

// 获取资源的周期
const defaultPeriod time.Duration = 1 * time.Second

type ResourceGetter interface {
}

type resourceGetter struct {
	ctx context.Context
	// 集群的 config 文件
	kubeconfig *rest.Config
	// 用来获取 resource 的两个客户端
	clientset             *kubernetes.Clientset
	dcClient              *discovery.DiscoveryClient
	sharedInformerFactory informers.SharedInformerFactory

	// 集群支持的 gvr, 及其对应的 GenericInformer
	gvrs      []schema.GroupVersionResource
	informers map[schema.GroupVersionResource]informers.GenericInformer

	// store 的更新时间
	period time.Duration
}

func NewResourceGetter(ctx context.Context, config *rest.Config) *resourceGetter {
	rg := &resourceGetter{
		ctx:        ctx,
		kubeconfig: config,
		period:     defaultPeriod,
	}

	rg.NewClientSet()
	rg.NewDiscoveryClient()
	rg.NewSharedInformerFactory()
	rg.GetGVRs()
	rg.NewInformersForResource()

	return rg
}

// 获取 config
func NewConfig() (*rest.Config, error) {
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

	return config, nil
}

// 根据 config 生成 client
func (rg *resourceGetter) NewClientSet() error {
	clientSet, err := kubernetes.NewForConfig(rg.kubeconfig)
	if err != nil {
		klog.Fatalf("build clientSet failed, err: ", err)
		return err
	}

	rg.clientset = clientSet
	return nil
}

// 根据 config 生成 DiscoveryClient
func (rg *resourceGetter) NewDiscoveryClient() error {
	dcClient, err := discovery.NewDiscoveryClientForConfig(rg.kubeconfig)
	if err != nil {
		klog.Fatalf("build dcClient failed, err: ", err)
		return err
	}

	rg.dcClient = dcClient
	return nil
}

// 生成 sharedInformerFactory
func (rg *resourceGetter) NewSharedInformerFactory() {
	informerFactory := informers.NewSharedInformerFactory(rg.clientset, 0)
	rg.sharedInformerFactory = informerFactory
}

// 对资源生成 informer
func (rg *resourceGetter) NewInformersForResource() error {
	informers := make(map[schema.GroupVersionResource]informers.GenericInformer, len(rg.gvrs))

	for _, gvr := range rg.gvrs {
		informer, err := rg.sharedInformerFactory.ForResource(gvr)
		if err != nil {
			klog.Errorf("create informer for resource: %+v failed", gvr)
			continue
		}

		informers[gvr] = informer
	}

	rg.informers = informers
	return nil
}

// 获取集群所有支持的 gvr
// 这里的 GVR 的 R 全是用的 plural name，后续解析 api 请求的时候需要考虑到
func (rg *resourceGetter) GetGVRs() error {
	lists, err := rg.dcClient.ServerPreferredResources()
	if err != nil {
		return err
	}

	resources := []schema.GroupVersionResource{}

	for _, list := range lists {
		//如果apiReosurce为空则跳过
		if len(list.APIResources) == 0 {
			continue
		}
		//解析GroupVersion
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}
		for _, resource := range list.APIResources {
			resources = append(resources, schema.GroupVersionResource{
				Group:    gv.Group,
				Version:  gv.Version,
				Resource: resource.Name})
		}
	}

	rg.gvrs = resources
	return nil
}

// 获取 resource
func (rg *resourceGetter) ListResources(informer informers.GenericInformer) ([]runtime.Object, error) {
	lister := informer.Lister()
	objs, err := lister.List(labels.Everything())
	if err != nil {
		klog.Errorf("list resource failed")
		return nil, err
	}

	return objs, nil
}

// 使用 DiscoveryClient 获取 GVK 对应的 GVR
func GVKToGVR(dcClient *discovery.DiscoveryClient, gvk schema.GroupVersionKind) (*schema.GroupVersionResource, error) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(dcClient))

	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		klog.Warningf("create mapping failed, err: ", err)
		return nil, err
	}

	return &mapping.Resource, nil
}

// TODO: 根据 http 请求解析 gvr
func ParseHttp() (schema.GroupVersionResource, error) {
	var resource schema.GroupVersionResource
	return resource, nil
}
