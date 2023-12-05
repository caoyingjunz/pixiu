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
	"strings"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	restclient "k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"

	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/client"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/caoyingjunz/pixiu/pkg/util/uuid"
)

type ClusterGetter interface {
	Cluster() Interface
}

type Interface interface {
	Create(ctx context.Context, clu *types.Cluster) error
	Update(ctx context.Context, cid int64, clu *types.Cluster) error
	Delete(ctx context.Context, cid int64) error
	Get(ctx context.Context, cid int64) (*types.Cluster, error)
	List(ctx context.Context) ([]types.Cluster, error)

	// Ping 检查和 k8s 集群的连通性
	Ping(ctx context.Context, kubeConfig string) error
	// AggregateEvents 聚合指定资源的 events
	AggregateEvents(ctx context.Context, cluster string, namespace string, name string, kind string) ([]types.Event, error)

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

func (c *cluster) preCreate(ctx context.Context, clu *types.Cluster) error {
	if len(clu.KubeConfig) == 0 {
		return fmt.Errorf("创建 kubernetes 集群时， kubeconfig 不允许为空")
	}

	// 实际创建前，先创建集群的连通性
	if err := c.Ping(ctx, clu.KubeConfig); err != nil {
		return fmt.Errorf("尝试连接 kubernetes API 失败: %v", err)
	}

	return nil
}

func (c *cluster) Create(ctx context.Context, clu *types.Cluster) error {
	if err := c.preCreate(ctx, clu); err != nil {
		return err
	}
	// TODO: 集群名称必须是由英文，数字组成
	if len(clu.Name) == 0 {
		clu.Name = uuid.NewRandName(8)
	}

	// 执行创建
	object, err := c.factory.Cluster().Create(ctx, &model.Cluster{
		Name:        clu.Name,
		AliasName:   clu.AliasName,
		ClusterType: int(clu.ClusterType),
		KubeConfig:  clu.KubeConfig,
		Description: clu.Description,
	})
	if err != nil {
		return err
	}

	cs, err := client.NewClusterSet(clu.KubeConfig)
	if err != nil {
		_ = c.Delete(ctx, object.Id)
		return err
	}

	// TODO: 暂时不做创建后动作
	clusterIndexer.Set(clu.Name, *cs)
	return nil
}

// TODO:
func (c *cluster) postCreate(ctx context.Context, cid int64, clu *types.Cluster) error {
	return nil
}

func (c *cluster) Update(ctx context.Context, cid int64, clu *types.Cluster) error {
	return c.factory.Cluster().Update(ctx, cid, clu.ResourceVersion, map[string]interface{}{
		"alias_name":  clu.AliasName,
		"description": clu.Description,
	})
}

// 删除前置检查
func (c *cluster) preDelete(ctx context.Context, cid int64) error {
	// TODO
	return nil
}

func (c *cluster) Delete(ctx context.Context, cid int64) error {
	if err := c.preDelete(ctx, cid); err != nil {
		return err
	}

	object, err := c.factory.Cluster().Delete(ctx, cid)
	if err != nil {
		return err
	}

	// 从缓存中移除 clusterSet
	clusterIndexer.Delete(object.Name)
	return nil
}

func (c *cluster) Get(ctx context.Context, cid int64) (*types.Cluster, error) {
	object, err := c.factory.Cluster().Get(ctx, cid)
	if err != nil {
		return nil, err
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

// AggregateEvents 聚合 k8s 资源的所有 events，比如 kind 为 deployment 时，则聚合 deployment，所属 rs 以及 pod 的事件
func (c *cluster) AggregateEvents(ctx context.Context, cluster string, namespace string, name string, kind string) ([]types.Event, error) {
	clusterSet, err := c.GetClusterSetByName(ctx, cluster)
	if err != nil {
		return nil, err
	}

	var fieldSelectors []string

	switch kind {
	case "deployment":
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

		// 获取 rs
		allReplicaSets, err := clusterSet.Client.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector, Limit: 500})
		if err != nil {
			return nil, err
		}
		var replicaSetUIDs []apitypes.UID
		for _, rs := range allReplicaSets.Items {
			for _, ownerReference := range rs.OwnerReferences {
				if ownerReference.Kind == "Deployment" && ownerReference.UID == deployment.UID {
					fieldSelectors = append(fieldSelectors, c.makeFieldSelector(rs.UID, rs.Name, rs.Namespace, "ReplicaSet"))
					replicaSetUIDs = append(replicaSetUIDs, rs.UID)
				}
			}
		}

		// 获取 pods
		allPods, err := clusterSet.Client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector, Limit: 500})
		if err != nil {
			return nil, err
		}
		for _, p := range allPods.Items {
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

	var allEvents []types.Event
	for _, fieldSelector := range fieldSelectors {
		events, err := clusterSet.Client.CoreV1().Events(namespace).List(context.TODO(), metav1.ListOptions{
			FieldSelector: fieldSelector,
			Limit:         500,
		})
		if err != nil {
			return nil, err
		}

		for _, event := range events.Items {
			allEvents = append(allEvents, types.Event{
				Type:    event.Type,
				Reason:  event.Reason,
				Message: event.Message,
			})
		}
	}

	return allEvents, nil
}

func (c *cluster) makeFieldSelector(uid apitypes.UID, name string, namespace string, kind string) string {
	return strings.Join([]string{
		"involvedObject.uid=" + string(uid),
		"involvedObject.name=" + name,
		"involvedObject.namespace=" + namespace,
		"involvedObject.kind=" + kind,
	}, ",")
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
	return types.Resources{Cpu: cpuSum.String(), Memory: memSum.String()}
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
		ClusterType: types.ClusterType(o.ClusterType),
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
