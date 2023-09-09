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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/client"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

const (
	pingNamespace = "kube-system"
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
	// TODO: 集群名称必须是由英文，数字组成
	if len(clu.Name) == 0 {
		return fmt.Errorf("创建 kubernetes 集群时，集群名称不允许为空")
	}

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

	return model2Type(object), nil
}

func (c *cluster) List(ctx context.Context) ([]types.Cluster, error) {
	objects, err := c.factory.Cluster().List(ctx)
	if err != nil {
		return nil, err
	}

	var cs []types.Cluster
	for _, object := range objects {
		cs = append(cs, *model2Type(&object))
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
	if _, err = clientSet.CoreV1().Namespaces().Get(ctx, pingNamespace, metav1.GetOptions{}); err != nil {
		return err
	}
	return nil
}

func (c *cluster) GetKubeConfigByName(ctx context.Context, name string) (*restclient.Config, error) {
	// 尝试从缓存中获取
	kubeConfig, ok := clusterIndexer.GetConfig(name)
	if ok {
		return kubeConfig, nil
	}

	// 缓存中不存在，则新建并重写回缓存
	object, err := c.factory.Cluster().GetClusterByName(ctx, name)
	if err != nil {
		return nil, err
	}
	cs, err := client.NewClusterSet(object.KubeConfig)
	if err != nil {
		return nil, err
	}

	clusterIndexer.Set(name, *cs)
	return cs.Config, nil
}

// GetKubernetesMeta
// TODO：临时构造 client，后续通过 informer 的方式维护缓存
func GetKubernetesMeta(ctx context.Context, clusterName string) (*types.KubernetesMeta, error) {
	clientSet, ok := clusterIndexer.GetClient(clusterName)
	if !ok {
		klog.Warningf("cluster %s does not register clientSet", clusterName)
		return nil, fmt.Errorf("failed to get %s clientSet from cluster indexer", clusterName)
	}

	// 获取 k8s 的节点信息
	nodeList, err := clientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	nodes := nodeList.Items
	// 在集群启动，但是没有节点加入时，命中该场景
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes found")
	}

	// 构造 kubernetes 元数据格式
	// TODO: 后续通过 informer 机制构造缓存
	// TODO: 补充集群资源数据
	return &types.KubernetesMeta{
		Nodes:             len(nodes),
		KubernetesVersion: nodes[0].Status.NodeInfo.KubeletVersion,
	}, nil
}

func model2Type(o *model.Cluster) *types.Cluster {
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
	kubernetesMeta, err := GetKubernetesMeta(context.TODO(), o.Name)
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
