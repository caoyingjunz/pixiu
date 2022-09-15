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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/core/client"
	pixiukubernetes "github.com/caoyingjunz/gopixiu/pkg/core/kubernetes"
	"github.com/caoyingjunz/gopixiu/pkg/db"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	"github.com/caoyingjunz/gopixiu/pkg/log"
)

var clientError = fmt.Errorf("failed to found clout client")

type CloudGetter interface {
	Cloud() CloudInterface
}

type CloudInterface interface {
	Create(ctx context.Context, obj *types.Cloud) error
	Update(ctx context.Context, obj *types.Cloud) error
	Delete(ctx context.Context, cid int64) error
	Get(ctx context.Context, cid int64) (*types.Cloud, error)
	List(ctx context.Context) ([]types.Cloud, error)

	Init() error // 初始化 cloud 的客户端

	// kubernetes 资源的接口定义
	pixiukubernetes.NamespacesGetter
	pixiukubernetes.ServicesGetter
	pixiukubernetes.StatefulSetGetter
	pixiukubernetes.DeploymentsGetter
	pixiukubernetes.DaemonSetGetter
	pixiukubernetes.JobsGetter
	pixiukubernetes.NodesGetter
}

var clientSets client.ClientsInterface

type cloud struct {
	app     *pixiu
	factory db.ShareDaoFactory
}

func newCloud(c *pixiu) CloudInterface {
	return &cloud{
		app:     c,
		factory: c.factory,
	}
}

func (c *cloud) preCreate(ctx context.Context, obj *types.Cloud) error {
	if len(obj.Name) == 0 {
		return fmt.Errorf("invalid empty cloud name")
	}
	if len(obj.KubeConfig) == 0 {
		return fmt.Errorf("invalid empty kubeconfig data")
	}
	// 集群类型支持 自建和标准，默认为标准
	// TODO: 未对类型进行检查
	if len(obj.CloudType) == 0 {
		obj.CloudType = "标准"
	}

	return nil
}

func (c *cloud) Create(ctx context.Context, obj *types.Cloud) error {
	if err := c.preCreate(ctx, obj); err != nil {
		log.Logger.Errorf("failed to pre-check for %a created: %v", obj.Name, err)
		return err
	}

	// 先构造 clientSet，如果有异常，直接返回
	clientSet, err := c.newClientSet(obj.KubeConfig)
	if err != nil {
		log.Logger.Errorf("failed to create %s clientSet: %v", obj.Name, err)
		return err
	}
	// 获取 k8s 集群信息: k8s 版本，节点数量，资源信息
	nodes, err := clientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil || len(nodes.Items) == 0 {
		log.Logger.Errorf("failed to connected to k8s cluster: %v", err)
		return err
	}

	var (
		kubeVersion string
		resources   string
	)
	// 第一个节点的版本作为集群版本
	node := nodes.Items[0]
	nodeStatus := node.Status
	kubeVersion = nodeStatus.NodeInfo.KubeletVersion
	// TODO: 未处理 resources
	if _, err = c.factory.Cloud().Create(ctx, &model.Cloud{
		Name:        obj.Name,
		CloudType:   obj.CloudType,
		KubeVersion: kubeVersion,
		KubeConfig:  string(obj.KubeConfig),
		NodeNumber:  len(nodes.Items),
		Resources:   resources,
	}); err != nil {
		log.Logger.Errorf("failed to create %s cloud: %v", obj.Name, err)
		return err
	}

	clientSets.Add(obj.Name, clientSet)
	return nil
}

func (c *cloud) Update(ctx context.Context, obj *types.Cloud) error { return nil }

func (c *cloud) Delete(ctx context.Context, cid int64) error {
	// TODO: 删除cloud的同时，直接返回，避免一次查询
	obj, err := c.factory.Cloud().Get(ctx, cid)
	if err != nil {
		log.Logger.Errorf("failed to get %s cloud: %v", cid, err)
		return err
	}
	if err = c.factory.Cloud().Delete(ctx, cid); err != nil {
		log.Logger.Errorf("failed to delete %s cloud: %v", cid, err)
		return err
	}

	clientSets.Delete(obj.Name)
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

func (c *cloud) List(ctx context.Context) ([]types.Cloud, error) {
	cloudObjs, err := c.factory.Cloud().List(ctx)
	if err != nil {
		log.Logger.Errorf("failed to list clouds: %v", err)
		return nil, err
	}
	var cs []types.Cloud
	for _, cloudObj := range cloudObjs {
		cs = append(cs, *c.model2Type(&cloudObj))
	}

	return cs, nil
}

func (c *cloud) Init() error {
	// 初始化云客户端
	clientSets = client.NewCloudClients()

	cloudObjs, err := c.factory.Cloud().List(context.TODO())
	if err != nil {
		log.Logger.Errorf("failed to list exist clouds: %v", err)
		return err
	}
	for _, cloudObj := range cloudObjs {
		clientSet, err := c.newClientSet([]byte(cloudObj.KubeConfig))
		if err != nil {
			log.Logger.Errorf("failed to create %s clientSet: %v", cloudObj.Name, err)
			return err
		}
		clientSets.Add(cloudObj.Name, clientSet)
	}

	return nil
}

func (c *cloud) newClientSet(data []byte) (*kubernetes.Clientset, error) {
	kubeConfig, err := clientcmd.RESTConfigFromKubeConfig(data)
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(kubeConfig)
}

func (c *cloud) model2Type(obj *model.Cloud) *types.Cloud {
	// TODO: 优化转换
	status := "正常"
	if obj.Status == 2 {
		status = "异常"
	}

	return &types.Cloud{
		Id:          obj.Id,
		Name:        obj.Name,
		Status:      status,
		CloudType:   obj.CloudType,
		KubeVersion: obj.KubeVersion,
		NodeNumber:  obj.NodeNumber,
		Resources:   obj.Resources,
		Description: obj.Description,
		TimeSpec: types.TimeSpec{
			GmtCreate:   obj.GmtCreate.Format(timeLayout),
			GmtModified: obj.GmtModified.Format(timeLayout),
		},
	}
}
