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

package core

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	pixiumeta "github.com/caoyingjunz/pixiu/api/meta"
	"github.com/caoyingjunz/pixiu/api/types"
	"github.com/caoyingjunz/pixiu/pkg/cache"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/log"
	pixiutypes "github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/caoyingjunz/pixiu/pkg/util"
	"github.com/caoyingjunz/pixiu/pkg/util/cipher"
	"github.com/caoyingjunz/pixiu/pkg/util/intstr"
)

type CloudGetter interface {
	Cloud() CloudInterface
}

type CloudInterface interface {
	// Load 上传已存在的 kubernetes 集群，直接导入 kubeConfig 文件
	Load(ctx context.Context, obj *types.LoadCloud) error
	// Build 自建 kubernetes 集群，需要更多的集群配置
	Build(ctx context.Context, obj *types.BuildCloud) error

	Delete(ctx context.Context, cid int64) error
	Get(ctx context.Context, cid int64) (*types.Cloud, error)
	List(ctx context.Context, selector *pixiumeta.ListSelector) (interface{}, error)

	// Restore 加载已经存在的 cloud 客户端
	Restore(ctx context.Context) error
	// SyncStatus 集群状态，
	SyncStatus(ctx context.Context, stopCh chan struct{})

	// Ping 检查 kubeConfig 与 kubernetes 集群的连通状态
	Ping(ctx context.Context, kubeConfigData []byte) error
	// GetClusterConfig 获取 kubeconfig 对象
	GetClusterConfig(ctx context.Context, clusterName string) (*restclient.Config, bool)
}

var (
	clusterSets cache.ClustersStore
)

type cloud struct {
	app     *pixiu
	factory db.ShareDaoFactory

	store cache.ClustersStore
}

func newCloud(c *pixiu) CloudInterface {
	return &cloud{
		app:     c,
		factory: c.factory,
	}
}

func (c *cloud) preLoad(ctx context.Context, obj *types.LoadCloud) error {
	if len(obj.AliasName) == 0 {
		return fmt.Errorf("invalid empty alias cloud name")
	}
	if len(obj.RawData) == 0 {
		return fmt.Errorf("invalid empty kubeconfig data")
	}
	// 集群类型支持 自建和标准，默认为标准

	// TODO: 其他规范创建前检查

	return nil
}

func (c *cloud) Load(ctx context.Context, obj *types.LoadCloud) error {
	if err := c.preLoad(ctx, obj); err != nil {
		return err
	}
	name := obj.Name
	cs, err := util.NewCloudSet(obj.RawData)
	if err != nil {
		return fmt.Errorf("failed to new %s clusterSet: %v", name, err)
	}
	// 直接对 kubeConfig 内容进行加密
	encryptData, err := cipher.Encrypt(obj.RawData)
	if err != nil {
		log.Logger.Errorf("failed to encrypt kubeConfig: %v", err)
		return err
	}

	cloudObj, err := c.factory.Cloud().Create(ctx, &model.Cloud{
		Name:      util.NewCloudName(pixiutypes.StandardCloudPrefix),
		AliasName: obj.Name,
		CloudType: pixiutypes.StandardCloud,
		Status:    pixiutypes.RunningStatus,
	})
	if err != nil {
		log.Logger.Errorf("failed to create %s cloud: %v", obj.Name, err)
		return err
	}
	// kubeConfig 数据落盘
	if _, err = c.factory.KubeConfig().Create(ctx, &model.KubeConfig{
		ServiceAccount: cloudObj.Name,
		ClusterRole:    "cloud-admin",
		CloudName:      cloudObj.Name,
		CloudId:        cloudObj.Id,
		Config:         encryptData,
	}); err != nil {
		log.Logger.Errorf("failed to save kubeConfig: %v", err)
		_, _ = c.factory.Cloud().Delete(ctx, cloudObj.Id)
		return err
	}

	clusterSets.Set(cloudObj.Name, *cs)
	return nil
}

func (c *cloud) preBuild(ctx context.Context, obj *types.BuildCloud) error {
	return nil
}

// Build 构造 Cloud
func (c *cloud) Build(ctx context.Context, obj *types.BuildCloud) error {
	if err := c.preBuild(ctx, obj); err != nil {
		log.Logger.Errorf("failed to pre-check for %s build: %v", obj.Name, err)
		return err
	}

	// step1: 创建 cloud
	cloudObj, err := c.factory.Cloud().Create(ctx, &model.Cloud{
		Name:        util.NewCloudName(pixiutypes.BuildCloudPrefix),
		AliasName:   obj.AliasName,
		Status:      pixiutypes.InitializeStatus, // 初始化状态
		CloudType:   pixiutypes.BuildCloud,
		KubeVersion: obj.Kubernetes.Version,
		Description: obj.Description,
	})
	if err != nil {
		log.Logger.Errorf("failed to create cloud %s: %v", obj.AliasName, err)
		return err
	}
	cid := cloudObj.Id

	// step2: 创建 k8s cluster
	if err = c.buildCluster(ctx, cid, obj.Kubernetes); err != nil {
		log.Logger.Errorf("failed to build %s cloud cluster: %v", obj.AliasName, err)
		_ = c.forceDelete(ctx, cid)
		return err
	}

	// step3: 创建 nodes
	if err = c.buildNodes(ctx, cid, obj.Kubernetes); err != nil {
		log.Logger.Errorf("failed to build %s cloud nodes: %v", obj.AliasName, err)
		_ = c.forceDelete(ctx, cid)
		return err
	}

	// 立刻进行部署
	if obj.Immediate {
		go c.StartDeployCluster(ctx, cid)
	}
	return nil
}

func (c *cloud) buildCluster(ctx context.Context, cid int64, kubeObj *types.KubernetesSpec) error {
	if err := c.factory.Cloud().CreateCluster(ctx, &model.Cluster{
		CloudId:     cid,
		ApiServer:   kubeObj.ApiServer,
		Version:     kubeObj.Version,
		Runtime:     kubeObj.Runtime,
		Cni:         kubeObj.Cni,
		ServiceCidr: kubeObj.ServiceCidr,
		PodCidr:     kubeObj.PodCidr,
		ProxyMode:   kubeObj.ProxyMode,
	}); err != nil {
		log.Logger.Errorf("failed to create cluster: %v", err)
		return err
	}

	return nil
}

func (c *cloud) buildNodes(ctx context.Context, cid int64, kubeObj *types.KubernetesSpec) error {
	var kubeNodes []model.Node
	// master 节点
	for _, master := range kubeObj.Masters {
		kubeNodes = append(kubeNodes, model.Node{
			CloudId:  cid,
			Role:     string(pixiutypes.MasterRole),
			HostName: master.HostName,
			Address:  master.Address,
			User:     master.User,
			Password: master.Password,
		})
	}
	// node 节点
	for _, node := range kubeObj.Nodes {
		kubeNodes = append(kubeNodes, model.Node{
			CloudId:  cid,
			Role:     string(pixiutypes.NodeRole),
			HostName: node.HostName,
			Address:  node.Address,
			User:     node.User,
			Password: node.Password,
		})
	}
	if err := c.factory.Cloud().CreateNodes(ctx, kubeNodes); err != nil {
		log.Logger.Errorf("failed to create %d nodes: %v", cid, err)
		return err
	}

	return nil
}

// 回滚 TODO
func (c *cloud) forceDelete(ctx context.Context, cid int64) error {
	return nil
}

func (c *cloud) Update(ctx context.Context, obj *types.Cloud) error { return nil }

// Delete 删除 cloud
// 1. 删除 kubeConfig
// 2. 同时根据 cloud 的类型，清除级联资源，标准资源直接删除，自建资源删除 cluster 和 nodes
func (c *cloud) Delete(ctx context.Context, cid int64) error {
	// 删除 kubeConfig
	if err := c.factory.KubeConfig().DeleteByCloud(ctx, cid); err != nil {
		log.Logger.Errorf("failed to delete %d kubeConfig content: %v", cid, err)
		return err
	}
	// 删除集群记录
	obj, err := c.factory.Cloud().Delete(ctx, cid)
	if err != nil {
		log.Logger.Errorf("failed to delete %s cloud: %v", cid, err)
		return err
	}

	clusterSets.Delete(obj.Name)

	// 目前，仅自建的k8s集群需要清理下属资源，下属资源有 cluster 和 nodes
	if obj.CloudType == pixiutypes.BuildCloud {
		_ = c.internalDelete(ctx, cid)
	}
	return nil
}

// TODO: 可以并发操作
// 清理 cloud 下属资源
func (c *cloud) internalDelete(ctx context.Context, cid int64) error {
	if err := c.factory.Cloud().DeleteCluster(ctx, cid); err != nil {
		return err
	}
	if err := c.factory.Cloud().DeleteNodes(ctx, cid); err != nil {
		return err
	}

	return nil
}

func (c *cloud) Get(ctx context.Context, cid int64) (*types.Cloud, error) {
	cloudObj, err := c.factory.Cloud().Get(ctx, cid)
	if err != nil {
		log.Logger.Errorf("failed to get %d cloud: %v", cid, err)
		return nil, err
	}

	return c.model2Type(cloudObj), nil
}

// List 返回分页查询
func (c *cloud) List(ctx context.Context, selector *pixiumeta.ListSelector) (interface{}, error) {
	cloudObjs, total, err := c.factory.Cloud().PageList(ctx, selector.Page, selector.Limit)
	if err != nil {
		log.Logger.Errorf("failed to list page %d size %d clouds: %v", selector.Page, selector.Limit, err)
		return nil, err
	}

	var cs []types.Cloud
	for _, cloudObj := range cloudObjs {
		cs = append(cs, *c.model2Type(&cloudObj))
	}

	return map[string]interface{}{
		"data":  cs,
		"total": total,
	}, nil
}

func (c *cloud) Ping(ctx context.Context, kubeConfigData []byte) error {
	// 先构造 clientSet，如果有异常，直接返回
	clientSet, err := util.NewClientSet(kubeConfigData)
	if err != nil {
		log.Logger.Errorf("failed to create clientSet: %v", err)
		return err
	}
	if _, err = clientSet.CoreV1().Namespaces().Get(ctx, "kube-system", metav1.GetOptions{}); err != nil {
		return err
	}

	return nil
}

func (c *cloud) Restore(ctx context.Context) error {
	// 初始化云客户端
	clusterSets = cache.ClustersStore{}

	// 获取待加载的 cloud 列表
	cloudObjs, err := c.factory.Cloud().List(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to list exist clouds: %v", err)
	}

	for _, cloudObj := range cloudObjs {
		// TODO: 仅加载状态正常的集群，异常的加入到异常列表
		if cloudObj.Status != 0 {
			continue
		}
		name := cloudObj.Name

		// Note:
		// 通过循环多次查询虽然增加了数据库查询次数，但是 cloud 本身数量可控，不会太多，且无需构造 map 对比，代码简洁
		configBytes, err := util.ParseKubeConfigData(context.TODO(), c.factory, intstr.FromInt64(cloudObj.Id))
		if err != nil {
			log.Logger.Errorf("failed to parse %d cloud kubeConfig: %v", name, err)
			return err
		}

		cs, err := util.NewCloudSet(configBytes)
		if err != nil {
			return err
		}

		clusterSets.Set(name, *cs)
		klog.V(2).Infof("restore clouds %s success", name)
	}

	return nil
}

// 间隔时间内获获取最新的集群列表，和缓存进行对比，如果有差异则进行更新
func (c *cloud) process(ctx context.Context) error {
	// 间隔时间内获获取最新的集群列表
	cs, err := c.factory.Cloud().List(ctx)
	if err != nil {
		return fmt.Errorf("failed to get exists clouds: %v", err)
	}

	fmt.Println(cs)
	return nil
}

// SyncStatus 定时同步集群状态
func (c *cloud) SyncStatus(ctx context.Context, stopCh chan struct{}) {
}

func (c *cloud) model2Type(obj *model.Cloud) *types.Cloud {
	return &types.Cloud{
		IdMeta: types.IdMeta{
			Id:              obj.Id,
			ResourceVersion: obj.ResourceVersion,
		},
		Name:        obj.Name,
		AliasName:   obj.AliasName,
		Status:      obj.Status,
		CloudType:   types.CloudType(obj.CloudType),
		KubeVersion: obj.KubeVersion,
		NodeNumber:  obj.NodeNumber,
		Resources:   obj.Resources,
		Description: obj.Description,
		TimeOption:  types.FormatTime(obj.GmtCreate, obj.GmtModified),
	}
}

// StartDeployCluster TODO
func (c *cloud) StartDeployCluster(ctx context.Context, cid int64) error {
	return nil
}

func (c *cloud) GetClusterConfig(ctx context.Context, clusterName string) (*restclient.Config, bool) {
	cluster, exists := clusterSets.Get(clusterName)
	if !exists {
		return nil, false
	}

	return cluster.KubeConfig, true
}
