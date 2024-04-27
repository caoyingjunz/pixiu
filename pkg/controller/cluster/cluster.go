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

package cluster

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"helm.sh/helm/v3/pkg/release"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog/v2"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/client"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/caoyingjunz/pixiu/pkg/util"
	"github.com/caoyingjunz/pixiu/pkg/util/uuid"
)

type ClusterGetter interface {
	Cluster() Interface
}

type Interface interface {
	Create(ctx context.Context, req *types.CreateClusterRequest) error
	Update(ctx context.Context, cid int64, req *types.UpdateClusterRequest) error
	Delete(ctx context.Context, cid int64) error
	Get(ctx context.Context, cid int64) (*types.Cluster, error)
	List(ctx context.Context) ([]types.Cluster, error)

	// Ping 检查和 k8s 集群的连通性
	Ping(ctx context.Context, kubeConfig string) error

	// Protect 设置集群的保护策略
	Protect(ctx context.Context, cid int64, req *types.ProtectClusterRequest) error

	// GetEventList 获取指定对象的事件，支持做聚合
	GetEventList(ctx context.Context, cluster string, options types.EventOptions) (*v1.EventList, error)

	// AggregateEvents 聚合指定资源的 events
	AggregateEvents(ctx context.Context, cluster string, namespace string, name string, kind string) (*v1.EventList, error)
	// WsHandler pod 的 webShell
	WsHandler(ctx context.Context, webShellOptions *types.WebShellOptions, w http.ResponseWriter, r *http.Request) error

	// ListReleases 获取 tenant release 列表
	ListReleases(ctx context.Context, cluster string, namespace string) ([]*release.Release, error)

	GetKubeConfigByName(ctx context.Context, name string) (*restclient.Config, error)
}

var clusterIndexer client.Cache

func init() {
	clusterIndexer = *client.NewClusterCache()
}

type cluster struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func (c *cluster) preCreate(ctx context.Context, req *types.CreateClusterRequest) error {
	// 实际创建前，先创建集群的连通性
	if err := c.Ping(ctx, req.KubeConfig); err != nil {
		return fmt.Errorf("尝试连接 kubernetes API 失败: %v", err)
	}
	return nil
}

func (c *cluster) Create(ctx context.Context, req *types.CreateClusterRequest) error {
	if err := c.preCreate(ctx, req); err != nil {
		return errors.NewError(err, http.StatusBadRequest)
	}
	// TODO: 集群名称必须是由英文，数字组成
	if len(req.Name) == 0 {
		req.Name = uuid.NewRandName(8)
	}

	var cs *client.ClusterSet
	var txFunc db.TxFunc = func() (err error) {
		cs, err = client.NewClusterSet(req.KubeConfig)
		return err
	}

	if _, err := c.factory.Cluster().Create(ctx, &model.Cluster{
		Name:        req.Name,
		AliasName:   req.AliasName,
		ClusterType: req.Type,
		Protected:   req.Protected,
		KubeConfig:  req.KubeConfig,
		Description: req.Description,
	}, txFunc); err != nil {
		klog.Errorf("failed to create cluster %s: %v", req.Name, err)
		return errors.ErrServerInternal
	}

	// TODO: 暂时不做创建后动作
	clusterIndexer.Set(req.Name, *cs)
	return nil
}

func (c *cluster) Update(ctx context.Context, cid int64, req *types.UpdateClusterRequest) error {
	object, err := c.factory.Cluster().Get(ctx, cid)
	if err != nil {
		klog.Errorf("failed to get cluster(%d): %v", cid, err)
		return errors.ErrServerInternal
	}
	if object == nil {
		return errors.ErrClusterNotFound
	}
	updates := make(map[string]interface{})
	if req.AliasName != nil {
		updates["alias_name"] = *req.AliasName
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if len(updates) == 0 {
		return errors.ErrInvalidRequest
	}
	if err := c.factory.Cluster().Update(ctx, cid, *req.ResourceVersion, updates); err != nil {
		klog.Errorf("failed to update cluster(%d): %v", cid, err)
		return errors.ErrServerInternal
	}
	return nil
}

// 删除前置检查
// 开启集群删除保护，则不允许删除
func (c *cluster) preDelete(ctx context.Context, cid int64) error {
	o, err := c.factory.Cluster().Get(ctx, cid)
	if err != nil {
		klog.Errorf("failed to get cluster(%d): %v", cid, err)
		return err
	}
	if o == nil {
		return errors.ErrClusterNotFound
	}
	// 开启集群删除保护，则不允许删除
	if o.Protected {
		return errors.NewError(fmt.Errorf("已开启集群删除保护功能，不允许删除 %s", o.AliasName), http.StatusForbidden)
	}

	// TODO: 其他删除策略检查
	return nil
}

func (c *cluster) Delete(ctx context.Context, cid int64) error {
	if err := c.preDelete(ctx, cid); err != nil {
		return err
	}
	object, err := c.factory.Cluster().Delete(ctx, cid)
	if err != nil {
		klog.Errorf("failed to delete cluster(%d): %v", cid, err)
		return errors.ErrServerInternal
	}

	// 从缓存中移除 clusterSet
	clusterIndexer.Delete(object.Name)
	return nil
}

func (c *cluster) Get(ctx context.Context, cid int64) (*types.Cluster, error) {
	object, err := c.factory.Cluster().Get(ctx, cid)
	if err != nil {
		return nil, errors.ErrServerInternal
	}
	if object == nil {
		return nil, errors.ErrClusterNotFound
	}

	return c.model2Type(object), nil
}

func (c *cluster) List(ctx context.Context) ([]types.Cluster, error) {
	objects, err := c.factory.Cluster().List(ctx)
	if err != nil {
		return nil, err
	}

	var cs []types.Cluster
	for _, object := range objects {
		cs = append(cs, *c.model2Type(&object))
	}

	return cs, nil
}

// Ping 检查和 k8s 集群的连通性
// 如果能获取到 k8s 接口的正常返回，则返回 nil，否则返回具体 error
// kubeConfig 为 k8s 证书的 base64 字符串
func (c *cluster) Ping(ctx context.Context, kubeConfig string) error {
	clientSet, err := client.NewClientSetFromString(kubeConfig)
	if err != nil {
		return err
	}

	// 调用 ns 资源，确保连通
	var timeout int64 = 1
	if _, err = clientSet.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
		TimeoutSeconds: &timeout,
	}); err != nil {
		klog.Errorf("failed to ping kubernetes: %v", err)
		// 处理原始报错信息，仅返回连接不通的信息
		return fmt.Errorf("kubernetes 集群连接测试失败")
	}

	return nil
}

func (c *cluster) Protect(ctx context.Context, cid int64, req *types.ProtectClusterRequest) error {
	if err := c.factory.Cluster().Update(ctx, cid, *req.ResourceVersion, map[string]interface{}{
		"protected": req.Protected,
	}); err != nil {
		klog.Errorf("failed to protect cluster(%d): %v", cid, err)
		return err
	}

	return nil
}

func (c *cluster) WsHandler(ctx context.Context, opt *types.WebShellOptions, w http.ResponseWriter, r *http.Request) error {
	cs, err := c.GetClusterSetByName(ctx, opt.Cluster)
	if err != nil {
		klog.Errorf("failed to get cluster(%s) client set: %v", opt.Cluster, err)
		return err
	}

	session, err := types.NewTerminalSession(w, r)
	if err != nil {
		return err
	}
	// 处理关闭
	defer func() {
		_ = session.Close()
	}()
	klog.Infof("connecting to %s/%s,", opt.Namespace, opt.Pod)

	cmd := opt.Command
	if len(cmd) == 0 {
		cmd = "/bin/bash"
	}

	// 组装 POST 请求
	req := cs.Client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(opt.Pod).
		Namespace(opt.Namespace).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Container: opt.Container,
			Command:   []string{cmd},
			Stderr:    true,
			Stdin:     true,
			Stdout:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

	// remotecommand 主要实现了http 转 SPDY 添加X-Stream-Protocol-Version相关header 并发送请求
	executor, err := remotecommand.NewSPDYExecutor(cs.Config, "POST", req.URL())
	if err != nil {
		return err
	}
	// 与 kubelet 建立 stream 连接
	if err = executor.Stream(remotecommand.StreamOptions{
		Stdout:            session,
		Stdin:             session,
		Stderr:            session,
		TerminalSizeQueue: session,
		Tty:               true,
	}); err != nil {
		_, _ = session.Write([]byte("exec pod command failed," + err.Error()))
		// 标记关闭terminal
		session.Done()
	}

	return nil
}

func (c *cluster) GetEventList(ctx context.Context, cluster string, options types.EventOptions) (*v1.EventList, error) {
	if options.Limit == 0 {
		options.Limit = 500
	}
	opt := metav1.ListOptions{Limit: options.Limit}
	fs := c.makeFieldSelector(apitypes.UID(options.Uid), options.Name, options.Namespace, options.Kind)
	if len(fs) != 0 {
		opt.FieldSelector = fs
	}

	clusterSet, err := c.GetClusterSetByName(ctx, cluster)
	if err != nil {
		return nil, err
	}

	return clusterSet.Client.CoreV1().Events(options.Namespace).List(ctx, opt)
}

// AggregateEvents 聚合 k8s 资源的所有 events，比如 kind 为 deployment 时，则聚合 deployment，所属 rs 以及 pod 的事件
func (c *cluster) AggregateEvents(ctx context.Context, cluster string, namespace string, name string, kind string) (*v1.EventList, error) {
	clusterSet, err := c.GetClusterSetByName(ctx, cluster)
	if err != nil {
		return nil, err
	}

	var fieldSelectors []string

	switch kind {
	case "deployment":
		// TODO: 临时聚合方式，后续继续优化（简化）
		// 获取 deployment
		deployment, err := clusterSet.Client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("failed to get deployment (%s/%s), err: %v", namespace, name, err)
			return nil, err
		}
		fieldSelectors = append(fieldSelectors, c.makeFieldSelector(deployment.UID, deployment.Name, deployment.Namespace, "Deployment"))

		var labels []string
		for k, v := range deployment.Spec.Selector.MatchLabels {
			labels = append(labels, fmt.Sprintf("%s=%s", k, v))
		}
		labelSelector := strings.Join(labels, ",")

		kubeObject, err := c.GetKubeObjectByLabel(clusterSet.Client, namespace, labelSelector, "ReplicaSet", "Pod")
		if err != nil {
			return nil, err
		}

		// 获取 rs
		allReplicaSets := kubeObject.GetReplicaSets()
		var replicaSetUIDs []apitypes.UID
		for _, rs := range allReplicaSets {
			for _, ownerReference := range rs.OwnerReferences {
				if ownerReference.Kind == "Deployment" && ownerReference.UID == deployment.UID {
					fieldSelectors = append(fieldSelectors, c.makeFieldSelector(rs.UID, rs.Name, rs.Namespace, "ReplicaSet"))
					replicaSetUIDs = append(replicaSetUIDs, rs.UID)
				}
			}
		}

		// 获取 pods
		allPods := kubeObject.GetPods()
		for _, p := range allPods {
			for _, ownerReference := range p.OwnerReferences {
				for _, replicaSetUID := range replicaSetUIDs {
					if ownerReference.UID == replicaSetUID && ownerReference.Kind == "ReplicaSet" {
						fieldSelectors = append(fieldSelectors, c.makeFieldSelector(p.UID, p.Name, p.Namespace, "Pod"))
					}
				}
			}
		}
	default:
		return nil, fmt.Errorf("unsupported kubernetes object kind %s", kind)
	}

	diff := len(fieldSelectors)
	errCh := make(chan error, diff)
	eventCh := make(chan *v1.EventList, diff)

	var wg sync.WaitGroup
	wg.Add(diff)
	for _, fieldSelector := range fieldSelectors {
		go func(fs string) {
			defer wg.Done()
			events, err := clusterSet.Client.CoreV1().Events(namespace).List(context.TODO(), metav1.ListOptions{
				FieldSelector: fs,
				Limit:         500,
			})
			if err != nil {
				klog.Errorf("failed to get object(%s) events: %v", namespace, err)
				errCh <- err
			}
			eventCh <- events
		}(fieldSelector)
	}
	wg.Wait()

	select {
	case err := <-errCh:
		if err != nil {
			return nil, err
		}
	default:
	}

	eventList := &v1.EventList{Items: []v1.Event{}}
	for i := 0; i < diff; i++ {
		es := <-eventCh
		if es == nil {
			continue
		}
		eventList.Items = append(eventList.Items, es.Items...)
	}

	return eventList, nil
}

// GetKubeObjectByLabel
// TODO: 并发优化
func (c *cluster) GetKubeObjectByLabel(Client *kubernetes.Clientset, namespace string, labelSelector string, kinds ...string) (*types.KubeObject, error) {
	object := &types.KubeObject{}

	kindSet := sets.NewString(kinds...)
	errCh := make(chan error, kindSet.Len())

	var wg sync.WaitGroup
	wg.Add(kindSet.Len())

	// 后续优化
	if kindSet.Has("ReplicaSet") {
		go func() {
			defer wg.Done()
			allReplicaSets, err := Client.AppsV1().ReplicaSets(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector, Limit: 500})
			if err != nil {
				errCh <- err
			} else {
				object.SetReplicaSets(allReplicaSets.Items)
			}
		}()
	}

	if kindSet.Has("Pod") {
		go func() {
			defer wg.Done()
			allPods, err := Client.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector, Limit: 500})
			if err != nil {
				errCh <- err
			} else {
				object.SetPods(allPods.Items)
			}
		}()
	}
	wg.Wait()

	select {
	case err := <-errCh:
		if err != nil {
			return nil, err
		}
	default:
	}
	return object, nil
}

func (c *cluster) GetKubeConfigByName(ctx context.Context, name string) (*restclient.Config, error) {
	cs, err := c.GetClusterSetByName(ctx, name)
	if err != nil {
		return nil, err
	}

	return cs.Config, nil
}

// GetClusterSetByName 获取 ClusterSet， 缓存中不存在时，构建缓存再返回
func (c *cluster) GetClusterSetByName(ctx context.Context, name string) (client.ClusterSet, error) {
	cs, ok := clusterIndexer.Get(name)
	if ok {
		klog.Infof("Get %s clusterSet from indexer", name)
		return cs, nil
	}

	klog.Infof("building clusterSet for %s", name)
	// 缓存中不存在，则新建并重写回缓存
	object, err := c.factory.Cluster().GetClusterByName(ctx, name)
	if err != nil {
		return client.ClusterSet{}, err
	}
	if object == nil {
		return client.ClusterSet{}, errors.ErrClusterNotFound
	}
	newClusterSet, err := client.NewClusterSet(object.KubeConfig)
	if err != nil {
		return client.ClusterSet{}, err
	}

	klog.Infof("set %s clusterSet into indexer", name)
	clusterIndexer.Set(name, *newClusterSet)
	return *newClusterSet, nil
}

// GetKubernetesMeta
// TODO：临时构造 client，后续通过 informer 的方式维护缓存
func (c *cluster) GetKubernetesMeta(ctx context.Context, clusterName string) (*types.KubernetesMeta, error) {
	clusterSet, err := c.GetClusterSetByName(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	// 获取 k8s 的节点信息
	nodeList, err := clusterSet.Client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	nodes := nodeList.Items
	// 在集群启动，但是没有节点加入时，命中该场景
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes found")
	}

	// 构造 kubernetes 资源数据格式
	// TODO: 后续通过 informer 机制构造缓存
	km := types.KubernetesMeta{
		Nodes:             len(nodes),
		KubernetesVersion: nodes[0].Status.NodeInfo.KubeletVersion,
	}

	// TODO: 并发优化
	// 获取集群所有节点的资源数据，并做整合
	metricList, err := clusterSet.Metric.NodeMetricses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	km.Resources = c.parseKubernetesResource(metricList.Items)

	return &km, nil
}

// 构造事件的 FieldSelector， 如果参数为空则忽略
func (c *cluster) makeFieldSelector(uid apitypes.UID, name string, namespace string, kind string) string {
	eventFS := make([]string, 0)
	// 追加对象的 uid
	if util.IsEmptyS(string(uid)) {
		eventFS = append(eventFS, "involvedObject.uid="+string(uid))
	}
	if util.IsEmptyS(name) {
		eventFS = append(eventFS, "involvedObject.name="+name)
	}
	if util.IsEmptyS(namespace) {
		eventFS = append(eventFS, "involvedObject.namespace="+namespace)
	}
	if util.IsEmptyS(kind) {
		eventFS = append(eventFS, "involvedObject.kind="+kind)
	}
	// 构造 kubernetes 原生 FieldSelector 参数格式
	return strings.Join(eventFS, ",")
}

func (c *cluster) parseKubernetesResource(nodeMetrics []v1beta1.NodeMetrics) types.Resources {
	// 初始化集群资源
	resourceList := v1.ResourceList{
		v1.ResourceCPU:    resource.Quantity{},
		v1.ResourceMemory: resource.Quantity{},
	}

	// 遍历所有 metric 数据，算集群总和，仅计算 cpu 和 memory
	for _, metric := range nodeMetrics {
		// 1. Cpu
		cpuMetric := metric.Usage.Cpu()
		if cpuMetric != nil {
			cpuSum := resourceList[v1.ResourceCPU]
			cpuSum.Add(*cpuMetric)
			resourceList[v1.ResourceCPU] = cpuSum
		}

		// 2. Memory
		memoryMetric := metric.Usage.Memory()
		if memoryMetric != nil {
			memSum := resourceList[v1.ResourceMemory]
			memSum.Add(*memoryMetric)
			resourceList[v1.ResourceMemory] = memSum
		}
	}

	cpuSum := resourceList[v1.ResourceCPU]
	memSum := resourceList[v1.ResourceMemory]
	return types.Resources{
		Cpu:    strconv.FormatFloat(parseFloat64FromString(cpuSum.String())/1000/1000/1000, 'f', 2, 64) + " Core",
		Memory: strconv.FormatFloat(parseFloat64FromString(memSum.String())/1024/1024, 'f', 2, 64) + " Gi"}
}

// parseFloat64FromString 从字符串中解析出包含的数字，并以 float64 返回。
// 无法解析时，返回 0
// 仅解析最先遇到的数字，效果：
// "666ddd" -> 666
// "666ddd888" -> 666
// "" 或者 "ddd"- > 0
func parseFloat64FromString(s string) float64 {
	matcher := regexp.MustCompile(`\d+`)
	fs := matcher.FindString(s)
	if len(fs) == 0 {
		return 0
	}

	f, err := strconv.ParseFloat(fs, 64)
	if err != nil {
		return 0
	}
	return f
}

func (c *cluster) model2Type(o *model.Cluster) *types.Cluster {
	tc := &types.Cluster{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
		Name:        o.Name,
		AliasName:   o.AliasName,
		ClusterType: o.ClusterType,
		Protected:   o.Protected,
		Description: o.Description,
	}

	// 获取失败时，返回空的 kubernetes Meta, 不终止主流程
	// TODO: 后续改成并发处理
	kubernetesMeta, err := c.GetKubernetesMeta(context.TODO(), o.Name)
	if err != nil {
		klog.Warning("failed to get kubernetes Meta: %v", err)
	} else {
		tc.KubernetesMeta = *kubernetesMeta
	}

	return tc
}

func NewCluster(cfg config.Config, f db.ShareDaoFactory) *cluster {
	return &cluster{
		cc:      cfg,
		factory: f,
	}
}
