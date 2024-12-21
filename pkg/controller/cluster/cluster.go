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
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/casbin/casbin/v2"
	"github.com/gorilla/websocket"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/client"
	ctrlutil "github.com/caoyingjunz/pixiu/pkg/controller/util"
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
	// WsNodeHandler node 的 webShell
	WsNodeHandler(ctx context.Context, sshConfig *types.WebSSHRequest, w http.ResponseWriter, r *http.Request) error

	// WatchPodLog 实时获取 pod 的日志
	WatchPodLog(ctx context.Context, cluster string, namespace string, podName string, containerName string, tailLine int64, w http.ResponseWriter, r *http.Request) error
	// ReRunJob 重新执行指定任务
	ReRunJob(ctx context.Context, cluster string, namespace string, jobName string, resourceVersion string) error

	GetKubeConfigByName(ctx context.Context, name string) (*restclient.Config, error)

	GetIndexerResource(ctx context.Context, cluster string, resource string, namespace string, name string) (interface{}, error)
	ListIndexerResources(ctx context.Context, cluster string, resource string, namespace string, listOption types.ListOptions) (interface{}, error)

	// Run 启动 cluster worker 处理协程
	Run(ctx context.Context, workers int) error
}

var ClusterIndexer client.Cache

func init() {
	ClusterIndexer = *client.NewClusterCache()
}

type (
	listerFunc func(ctx context.Context, informer *client.PixiuInformer, namespace string, listOption types.ListOptions) (interface{}, error)
	getterFunc func(ctx context.Context, informer *client.PixiuInformer, namespace, name string) (interface{}, error)
)

type InformerResource struct {
	// k8s 资源类型，比如 deployment, sts, daemonset 等
	ResourceType string
	ListerFunc   listerFunc
	GetterFunc   getterFunc
}

type cluster struct {
	cc       config.Config
	factory  db.ShareDaoFactory
	enforcer *casbin.SyncedEnforcer

	listerFuncs map[string]listerFunc
	getterFuncs map[string]getterFunc
}

func (c *cluster) preCreate(ctx context.Context, req *types.CreateClusterRequest) error {
	// 实际创建前，先创建集群的连通性
	if err := c.Ping(ctx, req.KubeConfig); err != nil {
		return fmt.Errorf("尝试连接 kubernetes API 失败: %v", err)
	}
	return nil
}

func (c *cluster) Create(ctx context.Context, req *types.CreateClusterRequest) error {
	user, err := httputils.GetUserFromRequest(ctx)
	if err != nil {
		return errors.NewError(err, http.StatusInternalServerError)
	}

	if err := c.preCreate(ctx, req); err != nil {
		return errors.NewError(err, http.StatusBadRequest)
	}
	// TODO: 集群名称必须是由英文，数字组成
	if len(req.Name) == 0 {
		req.Name = uuid.NewRandName(8)
	}

	var cs *client.ClusterSet
	var txFunc = func(cluster *model.Cluster) (err error) {
		if cs, err = client.NewClusterSet(req.KubeConfig); err != nil {
			return
		}

		// insert a user RBAC policy
		policy := model.NewPolicyFromModels(user, model.ObjectCluster, cluster.Model, model.OpAll)
		_, err = c.enforcer.AddPolicy(policy.Raw())
		return
	}

	kubeNode := types.KubeNode{}
	nodes, _ := kubeNode.Marshal()
	if _, err := c.factory.Cluster().Create(ctx, &model.Cluster{
		Name:        req.Name,
		AliasName:   req.AliasName,
		ClusterType: req.Type,
		Protected:   req.Protected,
		KubeConfig:  req.KubeConfig,
		Description: req.Description,
		Nodes:       nodes,
	}, txFunc); err != nil {
		klog.Errorf("failed to create cluster %s: %v", req.Name, err)
		return errors.ErrServerInternal
	}

	// TODO: 暂时不做创建后动作
	ClusterIndexer.Set(req.Name, *cs)
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
func (c *cluster) preDelete(ctx context.Context, cid int64) (cluster *model.Cluster, err error) {
	if cluster, err = c.factory.Cluster().Get(ctx, cid); err != nil {
		klog.Errorf("failed to get cluster(%d): %v", cid, err)
		return
	}
	if cluster == nil {
		return nil, errors.ErrClusterNotFound
	}
	// 开启集群删除保护，则不允许删除
	if cluster.Protected {
		return nil, errors.NewError(fmt.Errorf("已开启集群删除保护功能，不允许删除 %s", cluster.AliasName),
			http.StatusForbidden)
	}

	// TODO: 其他删除策略检查
	return
}

func (c *cluster) Delete(ctx context.Context, cid int64) error {
	user, err := httputils.GetUserFromRequest(ctx)
	if err != nil {
		return errors.NewError(err, http.StatusInternalServerError)
	}

	cluster, err := c.preDelete(ctx, cid)
	if err != nil {
		return err
	}

	var txFunc = func(cluster *model.Cluster) (err error) {
		_, err = c.enforcer.RemoveNamedPolicy("p", user.Name, model.ObjectCluster.String(), cluster.GetSID())
		return
	}
	if err := c.factory.Cluster().Delete(ctx, cluster, txFunc); err != nil {
		klog.Errorf("failed to delete cluster(%d): %v", cid, err)
		return errors.ErrServerInternal
	}

	// 从缓存中移除 clusterSet
	ClusterIndexer.Delete(cluster.Name)
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
	opts := ctrlutil.MakeDbOptions(ctx)
	objects, err := c.factory.Cluster().List(ctx, opts...)
	if err != nil {
		return nil, err
	}

	cs := make([]types.Cluster, len(objects))
	for i, object := range objects {
		cs[i] = *c.model2Type(&object)
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

// WatchPodLog streams the logs of a pod in a cluster to a websocket connection.
//
// Parameters:
// - ctx: The context.Context object for the request.
// - cluster: The name of the cluster.
// - namespace: The namespace of the pod.
// - podName: The name of the pod.
// - containerName: The name of the container.
// - tailLine: The number of lines to show from the end of the logs.
// - w: The http.ResponseWriter object for the websocket connection.
// - r: The *http.Request object for the websocket connection.
//
// Returns:
// - error: An error if there was a problem streaming the logs.
func (c *cluster) WatchPodLog(ctx context.Context, cluster string, namespace string, podName string, containerName string, tailLine int64, w http.ResponseWriter, r *http.Request) error {
	clusterSet, err := c.GetClusterSetByName(ctx, cluster)
	if err != nil {
		klog.Errorf("failed to get cluster(%s) clientSet: %v", cluster, err)
		return err
	}

	req := clusterSet.Client.CoreV1().Pods(namespace).GetLogs(podName, &v1.PodLogOptions{
		Container:  containerName,
		Follow:     true,
		TailLines:  &tailLine,
		Timestamps: false,
	})
	if req == nil {
		klog.Errorf("failed to get stream")
		return fmt.Errorf("failed to get stream")
	}

	withTimeout, cancelFunc := context.WithTimeout(ctx, time.Minute*10)
	defer cancelFunc()

	reader, err := req.Stream(withTimeout)
	if err != nil {
		klog.Errorf("failed to get stream: %v", err)
		return err
	}
	defer reader.Close()

	conn, err := util.BuildWebSocketConnection(w, r)
	if err != nil {
		klog.Errorf("failed to build websocket connection: %v", err)
		return err
	}
	defer conn.Close()

	for {
		buf := make([]byte, 1024)
		n, err := reader.Read(buf)
		if err != nil && err != io.EOF {
			break
		}
		err = conn.WriteMessage(websocket.TextMessage, buf[0:n])
		if err != nil {
			klog.Errorf("failed to write message: %v ,this websocket connection will be closed", err)
			break
		}
	}
	return nil
}

const Retries = 3

// ReRunJob 重新运行(创建)任务，通过先删除在创建的方式实现，极端情况下可能导致 job 丢失
func (c *cluster) ReRunJob(ctx context.Context, cluster string, namespace string, jobName string, resourceVersion string) error {
	cs, err := c.GetClusterSetByName(ctx, cluster)
	if err != nil {
		return err
	}

	job, err := cs.Client.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if job.ResourceVersion != resourceVersion {
		return fmt.Errorf("please apply your changes to the latest and re-run")
	}

	newJob := *job
	// 重置不必要字段
	newJob.ResourceVersion = ""
	newJob.ObjectMeta.UID = ""
	newJob.Status = batchv1.JobStatus{}
	// 重置 uid 和 label
	delete(newJob.Spec.Selector.MatchLabels, "controller-uid")
	delete(newJob.Spec.Selector.MatchLabels, "batch.kubernetes.io/controller-uid")
	delete(newJob.Spec.Template.ObjectMeta.Labels, "controller-uid")
	delete(newJob.Spec.Template.ObjectMeta.Labels, "batch.kubernetes.io/controller-uid")
	delete(newJob.Spec.Template.ObjectMeta.Labels, "batch.kubernetes.io/job-name")
	delete(newJob.Spec.Template.ObjectMeta.Labels, "job-name")

	// TODO: 备份一次job，避免失败job丢失
	// 2. 删除job
	if err = cs.Client.BatchV1().Jobs(namespace).Delete(ctx, jobName, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("failed to rerun job(%s) %v", jobName, err)
	}

	var jobErr error
	// 3. 新建job，最多重试 3 次
	for i := 0; i < Retries; i++ {
		_, jobErr = cs.Client.BatchV1().Jobs(namespace).Create(ctx, &newJob, metav1.CreateOptions{})
		if jobErr != nil {
			time.Sleep(time.Second)
			continue
		}
		break
	}
	if jobErr != nil {
		return fmt.Errorf("failed to rerun job(%s) %v", jobName, err)
	}

	return nil
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
	cs, ok := ClusterIndexer.Get(name)
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
	ClusterIndexer.Set(name, *newClusterSet)
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
	//metricList, err := clusterSet.Metric.NodeMetricses().List(ctx, metav1.ListOptions{})
	//if err != nil {
	//	return nil, err
	//}
	//km.Resources = c.parseKubernetesResource(metricList.Items)

	return &km, nil
}

func (c *cluster) GetKubernetesMetaFromPlan(ctx context.Context, planId int64) (*types.KubernetesMeta, error) {
	planConfig, err := c.factory.Plan().GetConfigByPlan(ctx, planId)
	if err != nil {
		return nil, err
	}
	ks := &types.KubernetesSpec{}
	if err = ks.Unmarshal(planConfig.Kubernetes); err != nil {
		return nil, err
	}

	nodes, err := c.factory.Plan().ListNodes(ctx, planId)
	if err != nil {
		return nil, err
	}

	return &types.KubernetesMeta{
		KubernetesVersion: "v" + ks.KubernetesVersion,
		Nodes:             len(nodes),
	}, nil
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
	nodes := types.KubeNode{}
	if err := nodes.Unmarshal(o.Nodes); err != nil {
		// 非核心数据
		klog.Warningf("failed to unmarshal cluster nodes: %v", err)
	}

	tc := &types.Cluster{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
		Name:              o.Name,
		AliasName:         o.AliasName,
		ClusterType:       o.ClusterType,
		KubernetesVersion: o.KubernetesVersion,
		Nodes:             nodes,
		PlanId:            o.PlanId,
		Status:            o.ClusterStatus, // 默认是运行中状态，自建集群会根据实际任务状态修改状态
		Protected:         o.Protected,
		Description:       o.Description,
	}

	//var (
	//	kubernetesMeta *types.KubernetesMeta
	//	err            error
	//)
	//
	//if o.ClusterType == model.ClusterTypeStandard {
	//	// 导入的集群通过API获取相关数据
	//	// 获取失败时，返回空的 kubernetes Meta, 不终止主流程
	//	// TODO: 后续改成并发处理
	//	kubernetesMeta, err = c.GetKubernetesMeta(context.TODO(), o.Name)
	//} else {
	//	// 自建的集群通过plan配置获取版本信息
	//	kubernetesMeta, err = c.GetKubernetesMetaFromPlan(context.TODO(), o.PlanId)
	//
	//	// 自建的集群需要从 plan task 获取状态
	//	tc.Status, _ = c.GetClusterStatusFromPlanTask(o.PlanId)
	//}
	//if err != nil {
	//	klog.Warning("failed to get kubernetes Meta: %v", err)
	//} else {
	//	tc.KubernetesMeta = *kubernetesMeta
	//}

	return tc
}

func (c *cluster) GetClusterStatusFromPlanTask(planId int64) (model.ClusterStatus, error) {
	status := model.ClusterStatusRunning

	// 尝试获取最新的任务状态
	// 获取失败也不中断返回
	if tasks, err := c.factory.Plan().ListTasks(context.TODO(), planId); err == nil {
		if len(tasks) == 0 {
			status = model.ClusterStatusUnStart
		} else {
			for _, task := range tasks {
				if task.Status != model.SuccessPlanStatus {
					if task.Status == model.FailedPlanStatus {
						status = model.ClusterStatusFailed
					} else {
						status = model.ClusterStatusDeploy
					}
					break
				}
			}
		}
	}

	return status, nil
}

func (c *cluster) registerIndexers(informerResources ...InformerResource) {
	for _, informerResource := range informerResources {
		c.listerFuncs[informerResource.ResourceType] = informerResource.ListerFunc
		c.getterFuncs[informerResource.ResourceType] = informerResource.GetterFunc
	}
}

func (c *cluster) Run(ctx context.Context, workers int) error {
	klog.Infof("starting cluster manager")
	// 同步集群状态，节点数，版本
	go wait.UntilWithContext(ctx, c.Sync, 5*time.Second)

	return nil
}

func (c *cluster) Sync(ctx context.Context) {
	// TODO: 后续添加同步任务
}

func NewCluster(cfg config.Config, f db.ShareDaoFactory, e *casbin.SyncedEnforcer) *cluster {
	c := &cluster{
		cc:       cfg,
		factory:  f,
		enforcer: e,

		listerFuncs: make(map[string]listerFunc),
		getterFuncs: make(map[string]getterFunc),
	}

	// TODO: code generation?
	c.registerIndexers([]InformerResource{
		{
			ResourceType: ResourcePod,
			ListerFunc: func(ctx context.Context, informer *client.PixiuInformer, namespace string, listOption types.ListOptions) (interface{}, error) {
				return c.ListPods(ctx, informer.PodsLister(), namespace, listOption)
			},
			GetterFunc: func(ctx context.Context, informer *client.PixiuInformer, namespace, name string) (interface{}, error) {
				return c.GetPod(ctx, informer.PodsLister(), namespace, name)
			},
		},
		{
			ResourceType: ResourceDeployment,
			ListerFunc: func(ctx context.Context, informer *client.PixiuInformer, namespace string, listOption types.ListOptions) (interface{}, error) {
				return c.ListDeployments(ctx, informer.DeploymentsLister(), namespace, listOption)
			},
			GetterFunc: func(ctx context.Context, informer *client.PixiuInformer, namespace, name string) (interface{}, error) {
				return c.GetDeployment(ctx, informer.DeploymentsLister(), namespace, name)
			},
		},
		{
			ResourceType: ResourceStatefulSet,
			ListerFunc: func(ctx context.Context, informer *client.PixiuInformer, namespace string, listOption types.ListOptions) (interface{}, error) {
				return c.ListStatefulSets(ctx, informer.StatefulSetsLister(), namespace, listOption)
			},
			GetterFunc: func(ctx context.Context, informer *client.PixiuInformer, namespace, name string) (interface{}, error) {
				return c.GetStatefulSet(ctx, informer.StatefulSetsLister(), namespace, name)
			},
		},
		{
			ResourceType: ResourceDaemonSet,
			ListerFunc: func(ctx context.Context, informer *client.PixiuInformer, namespace string, listOption types.ListOptions) (interface{}, error) {
				return c.ListDaemonSets(ctx, informer.DaemonSetsLister(), namespace, listOption)
			},
			GetterFunc: func(ctx context.Context, informer *client.PixiuInformer, namespace, name string) (interface{}, error) {
				return c.GetDaemonSet(ctx, informer.DaemonSetsLister(), namespace, name)
			},
		},
		{
			ResourceType: ResourceCronJob,
			ListerFunc: func(ctx context.Context, informer *client.PixiuInformer, namespace string, listOption types.ListOptions) (interface{}, error) {
				return c.ListCronJobs(ctx, informer.CronJobsLister(), namespace, listOption)
			},
			GetterFunc: func(ctx context.Context, informer *client.PixiuInformer, namespace, name string) (interface{}, error) {
				return c.GetCronJob(ctx, informer.CronJobsLister(), namespace, name)
			},
		},
		{
			ResourceType: ResourceJob,
			ListerFunc: func(ctx context.Context, informer *client.PixiuInformer, namespace string, listOption types.ListOptions) (interface{}, error) {
				return c.ListJobs(ctx, informer.JobsLister(), namespace, listOption)
			},
			GetterFunc: func(ctx context.Context, informer *client.PixiuInformer, namespace, name string) (interface{}, error) {
				return c.GetJob(ctx, informer.JobsLister(), namespace, name)
			},
		},
		{
			ResourceType: ResourceNode,
			ListerFunc: func(ctx context.Context, informer *client.PixiuInformer, namespace string, listOption types.ListOptions) (interface{}, error) {
				return c.ListNodes(ctx, informer.NodesLister(), namespace, listOption)
			},
			GetterFunc: func(ctx context.Context, informer *client.PixiuInformer, namespace, name string) (interface{}, error) {
				return c.GetNode(ctx, informer.NodesLister(), namespace, name)
			},
		},
		// TODO: 补充更多资源实现
	}...)
	return c
}
