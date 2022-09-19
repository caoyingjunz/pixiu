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
	"encoding/base64"
	"fmt"

	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

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

	Load() error // 加载已经存在的 cloud 客户端

	KubeConfig(ctx context.Context, cloudName string) (string, error)

	// kubernetes 资源的接口定义
	pixiukubernetes.NamespacesGetter
	pixiukubernetes.ServicesGetter
	pixiukubernetes.StatefulSetGetter
	pixiukubernetes.DeploymentsGetter
	pixiukubernetes.DaemonSetGetter
	pixiukubernetes.JobsGetter
	pixiukubernetes.NodesGetter
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

func (c *cloud) KubeConfig(ctx context.Context, cloudName string) (string, error) {
	if len(cloudName) == 0 {
		return "", fmt.Errorf("cloud_name is null")
	}

	var (
		saName                  = "gopixiu" // TODO
		serverAddr              = "https://39.100.127.217:6443"
		clientSet               = clientSets.Get(cloudName)
		expirationSeconds int64 = 2592000 // 1 month
		namespace               = "kube-system"
		sa                *corev1.ServiceAccount
	)

	// create sa
	sa, err := clientSet.CoreV1().ServiceAccounts(namespace).Get(ctx, saName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return "", err
	} else if errors.IsNotFound(err) {
		sa = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: saName,
			},
		}
		if _, err = clientSet.CoreV1().ServiceAccounts(namespace).Create(ctx, sa, metav1.CreateOptions{}); err != nil {
			return "", err
		}

		// TODO 当前只支持admin
		clusterRoleBinding := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: saName,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      saName,
					Namespace: namespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				Kind: "ClusterRole",
				Name: "cluster-admin",
			},
		}
		if _, err = clientSet.RbacV1().ClusterRoleBindings().Create(ctx, clusterRoleBinding, metav1.CreateOptions{}); err != nil {
			return "", err
		}
	}

	// ca
	cm, err := clientSet.CoreV1().ConfigMaps(namespace).Get(context.TODO(), "kube-root-ca.crt", metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	ca := base64.StdEncoding.EncodeToString([]byte(cm.Data["ca.crt"]))

	// create token
	tokenRequest := &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{
			ExpirationSeconds: &expirationSeconds,
		},
	}
	token, err := clientSet.CoreV1().ServiceAccounts(namespace).CreateToken(ctx, saName, tokenRequest, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}

	return newKubeConfig(serverAddr, ca, saName, token.Status.Token), nil
}

func (c *cloud) Load() error {
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

func newKubeConfig(server, ca, user, token string) string {
	const kubeConfigTpl = `apiVersion: v1
kind: Config
current-context: kubernetes
clusters:
- name: kubernetes
  cluster:
    server: %s
    certificate-authority-data: %s
contexts:
- name: kubernetes
  context:
    cluster: kubernetes
    user: %s
users:
- name: %s
  user:
    token: %s
`
	return fmt.Sprintf(kubeConfigTpl, server, ca, user, user, token)
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
