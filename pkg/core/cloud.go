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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/core/client"
	pixiukubernetes "github.com/caoyingjunz/gopixiu/pkg/core/kubernetes"
	"github.com/caoyingjunz/gopixiu/pkg/db"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	"github.com/caoyingjunz/gopixiu/pkg/log"
	"github.com/caoyingjunz/gopixiu/pkg/util/cipher"
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
	List(ctx context.Context, paging *types.PageOptions) (interface{}, error)

	Load(stopCh chan struct{}) error // 加载已经存在的 cloud 客户端

	// kubernetes 资源的接口定义
	pixiukubernetes.NamespacesGetter
	pixiukubernetes.ServicesGetter
	pixiukubernetes.StatefulSetGetter
	pixiukubernetes.DeploymentsGetter
	pixiukubernetes.DaemonSetGetter
	pixiukubernetes.JobsGetter
	pixiukubernetes.IngressGetter
	pixiukubernetes.EventsGetter
	pixiukubernetes.NodesGetter
	pixiukubernetes.PodsGetter
	pixiukubernetes.KubeConfigGetter
	pixiukubernetes.ScalesGetter
}

const (
	page     = 1
	pageSize = 10
)

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
		log.Logger.Errorf("failed to pre-check for %s created: %v", obj.Name, err)
		return err
	}
	// 直接对 kubeConfig 内容进行加密
	encryptData, err := cipher.Encrypt(obj.KubeConfig)
	if err != nil {
		log.Logger.Errorf("failed to encrypt kubeConfig: %v", err)
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
		KubeConfig:  encryptData,
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

// List 返回全量查询或者分页查询
func (c *cloud) List(ctx context.Context, pageOption *types.PageOptions) (interface{}, error) {
	var cs []types.Cloud

	// 根据是否有 Page 判断是全量查询还是分页查询，如果没有则判定为全量查询，如果有，判定为分页查询
	// TODO: 判断方法略显粗糙，后续优化
	// 分页查询
	if pageOption.Page != 0 {
		if pageOption.Page < 0 {
			pageOption.Page = page
		}
		if pageOption.Limit <= 0 {
			pageOption.Limit = pageSize
		}
		cloudObjs, total, err := c.factory.Cloud().PageList(ctx, pageOption.Page, pageOption.Limit)
		if err != nil {
			log.Logger.Errorf("failed to page %d limit %d list  clouds: %v", pageOption.Page, pageOption.Limit, err)
			return nil, err
		}
		// 类型转换
		for _, cloudObj := range cloudObjs {
			cs = append(cs, *c.model2Type(&cloudObj))
		}

		pageClouds := make(map[string]interface{})
		pageClouds["data"] = cs
		pageClouds["total"] = total
		return pageClouds, nil
	}

	// 全量查询
	cloudObjs, err := c.factory.Cloud().List(ctx)
	if err != nil {
		log.Logger.Errorf("failed to list clouds: %v", err)
		return nil, err
	}
	for _, cloudObj := range cloudObjs {
		cs = append(cs, *c.model2Type(&cloudObj))
	}

	return cs, nil
}

func (c *cloud) Load(stopCh chan struct{}) error {
	// 初始化云客户端
	clientSets = client.NewCloudClients()

	cloudObjs, err := c.factory.Cloud().List(context.TODO())
	if err != nil {
		log.Logger.Errorf("failed to list exist clouds: %v", err)
		return err
	}
	for _, cloudObj := range cloudObjs {
		kubeConfig, err := cipher.Decrypt(cloudObj.KubeConfig)
		if err != nil {
			return err
		}
		clientSet, err := c.newClientSet(kubeConfig)
		if err != nil {
			log.Logger.Errorf("failed to create %s clientSet: %v", cloudObj.Name, err)
			return err
		}
		clientSets.Add(cloudObj.Name, clientSet)
		klog.V(2).Infof("load cloud %s success", cloudObj.Name)
	}

	// 启动集群检查状态检查
	go c.ClusterHealthCheck(stopCh)
	return nil
}

func (c *cloud) ClusterHealthCheck(stopCh chan struct{}) {
	klog.V(2).Infof("starting cluster health check")
	status := make(map[string]int)

	interval := time.Second * 5
	for {
		select {
		case <-time.After(interval):
			for name, cs := range clientSets.List() {
				// TODO: 做并发优化
				// TODO: 请求的超时设置
				// TODO: 定时刷新 status 的存量
				var newStatus int
				if _, err := cs.CoreV1().Namespaces().Get(context.TODO(), "kube-system", metav1.GetOptions{}); err != nil {
					log.Logger.Errorf("failed to check %s cluster: %v", name, err)
					newStatus = 1
				}

				// 对比状态是否发生改变
				if status[name] != newStatus {
					status[name] = newStatus
					_ = c.factory.Cloud().SetStatus(context.TODO(), name, newStatus)
				}
			}
		case <-stopCh:
			klog.Infof("shutting cluster health check")
			return
		}
	}
}

func (c *cloud) newClientSet(data []byte) (*kubernetes.Clientset, error) {
	kubeConfig, err := clientcmd.RESTConfigFromKubeConfig(data)
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(kubeConfig)
}

func (c *cloud) model2Type(obj *model.Cloud) *types.Cloud {
	return &types.Cloud{
		Id:          obj.Id,
		Name:        obj.Name,
		Status:      obj.Status,
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
