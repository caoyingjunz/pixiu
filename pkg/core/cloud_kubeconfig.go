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
	"strconv"
	"time"

	"gopkg.in/yaml.v2"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/db"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	"github.com/caoyingjunz/gopixiu/pkg/log"
	"github.com/caoyingjunz/gopixiu/pkg/util/cipher"
)

func (c *cloud) KubeConfigs(cloud string) KubeConfigInterface {
	return NewKubeConfigs(clientSets.Get(cloud), cloud, c.factory)
}

type KubeConfigGetter interface {
	KubeConfigs(cloud string) KubeConfigInterface
}

type KubeConfigInterface interface {
	Create(ctx context.Context, kubeConfig *types.KubeConfig) (*types.KubeConfig, error)
	Update(ctx context.Context, kid int64) (*types.KubeConfig, error)
	Delete(ctx context.Context, kid int64) error
	Get(ctx context.Context, kid int64) (*types.KubeConfig, error)
	List(ctx context.Context, cloudName string) ([]types.KubeConfig, error)
}

type kubeConfigs struct {
	client  *kubernetes.Clientset
	cloud   string
	factory db.ShareDaoFactory
}

func NewKubeConfigs(client *kubernetes.Clientset, cloud string, factory db.ShareDaoFactory) KubeConfigInterface {
	return &kubeConfigs{
		client:  client,
		cloud:   cloud,
		factory: factory,
	}
}

const namespace = "kube-system"

// preCreate 创建前检查, 默认权限为 cluster-admin
func (c *kubeConfigs) preCreate(ctx context.Context, kubeConfig *types.KubeConfig) error {
	if len(kubeConfig.ServiceAccount) == 0 {
		kubeConfig.ServiceAccount = strconv.FormatInt(time.Now().Unix(), 10)
	}
	if len(kubeConfig.ClusterRole) == 0 {
		kubeConfig.ClusterRole = "cluster-admin"
	}

	return nil
}

func (c *kubeConfigs) Create(ctx context.Context, kubeConfig *types.KubeConfig) (*types.KubeConfig, error) {
	if c.client == nil {
		return nil, clientError
	}
	// 创建前检查
	if err := c.preCreate(ctx, kubeConfig); err != nil {
		return nil, err
	}
	// 获取集群信息
	cloudObj, err := c.factory.Cloud().GetByName(ctx, kubeConfig.CloudName)
	if err != nil {
		return nil, err
	}
	// 解密集群 config
	cloudConfigByte, err := cipher.Decrypt(cloudObj.KubeConfig)
	if err != nil {
		log.Logger.Errorf("failed to Decrypt cloud KubeConfig: %v", err)
		return nil, err
	}
	cloudConfig := newConfig()
	if err = yaml.Unmarshal(cloudConfigByte, cloudConfig); err != nil {
		return nil, err
	}

	// 创建 service account
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: kubeConfig.ServiceAccount,
		},
	}
	if _, err = c.client.CoreV1().ServiceAccounts(namespace).Create(ctx, sa, metav1.CreateOptions{}); err != nil {
		log.Logger.Errorf("failed to create service account: %v", err)
		return nil, err
	}

	// 创建 cluster role binding
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: kubeConfig.ServiceAccount,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      kubeConfig.ServiceAccount,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind: "ClusterRole",
			Name: kubeConfig.ClusterRole,
		},
	}
	if _, err = c.client.RbacV1().ClusterRoleBindings().Create(ctx, clusterRoleBinding, metav1.CreateOptions{}); err != nil {
		log.Logger.Errorf("failed to create cluster role binding: %v", err)
		return nil, err
	}

	// 获取server地址, ca
	serverAddr := cloudConfig.Clusters[0].Cluster.Server
	ca := cloudConfig.Clusters[0].Cluster.CertificateAuthorityData
	// 生成token
	token, err := c.createToken(ctx, kubeConfig.ServiceAccount)
	if err != nil {
		log.Logger.Errorf("failed to get token: %v", err)
		return nil, err
	}

	// 生成 config
	config := newConfig()
	config.setServer(serverAddr)
	config.setCA(ca)
	config.setSA(kubeConfig.ServiceAccount, token.Status.Token)
	configByte, err := yaml.Marshal(config)
	if err != nil {
		log.Logger.Error(err)
		return nil, err
	}

	kubeConfig.Config = string(configByte)
	kubeConfig.ExpirationTimestamp = token.Status.ExpirationTimestamp.String()

	// 写库, kubeConfig 内容进行加密
	obj := c.type2Model(kubeConfig)
	obj.Config, err = cipher.Encrypt(configByte)
	if err != nil {
		log.Logger.Errorf("failed to encrypt kubeConfig: %v", err)
		return nil, err
	}
	obj, err = c.factory.KubeConfig().Create(ctx, obj)
	if err != nil {
		log.Logger.Errorf("failed to create kubeConfig: %v", err)
		return nil, err
	}
	kubeConfig.Id = obj.Id

	return kubeConfig, nil
}

func (c *kubeConfigs) Update(ctx context.Context, kid int64) (*types.KubeConfig, error) {
	if c.client == nil {
		return nil, clientError
	}
	obj, err := c.factory.KubeConfig().Get(ctx, kid)
	if err != nil {
		log.Logger.Errorf("failed to get kubeConfig: %v", err)
		return nil, err
	}
	// 解密 config
	configByte, err := cipher.Decrypt(obj.Config)
	if err != nil {
		log.Logger.Errorf("failed to decrypt kubeConfig: %v", err)
		return nil, err
	}
	// 生成 config 对象
	config := newConfig()
	if err = yaml.Unmarshal(configByte, config); err != nil {
		log.Logger.Error(err)
		return nil, err
	}
	// 设置新token
	token, err := c.createToken(ctx, obj.ServiceAccount)
	if err != nil {
		log.Logger.Errorf("failed to get token: %v", err)
		return nil, err
	}
	config.setSA(obj.ServiceAccount, token.Status.Token)

	// 写库, kubeConfig 内容进行加密
	newConfigByte, err := yaml.Marshal(config)
	if err != nil {
		log.Logger.Error(err)
		return nil, err
	}
	newConfigEncryptStr, err := cipher.Encrypt(newConfigByte)
	if err != nil {
		log.Logger.Errorf("failed to encrypt kubeConfig: %v", err)
		return nil, err
	}
	if err = c.factory.KubeConfig().Update(ctx, kid, obj.ResourceVersion+1,
		map[string]interface{}{"config": newConfigEncryptStr},
	); err != nil {
		log.Logger.Errorf("failed to update kubeConfig: %v", err)
		return nil, err
	}

	kubeConfig := c.model2Type(obj)
	kubeConfig.Config = string(newConfigByte)
	kubeConfig.ExpirationTimestamp = token.Status.ExpirationTimestamp.String()

	return kubeConfig, nil
}

func (c *kubeConfigs) Delete(ctx context.Context, kid int64) error {
	if c.client == nil {
		return clientError
	}
	obj, err := c.factory.KubeConfig().Get(ctx, kid)
	if err != nil {
		return err
	}
	if err = c.client.RbacV1().ClusterRoleBindings().Delete(ctx, obj.ServiceAccount, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		log.Logger.Errorf("failed to delete cluster role binding: %v", err)
		return err
	}
	if err = c.client.CoreV1().ServiceAccounts(namespace).Delete(ctx, obj.ServiceAccount, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		log.Logger.Errorf("failed to delete service account: %v", err)
		return err
	}
	if err = c.factory.KubeConfig().Delete(ctx, kid); err != nil {
		log.Logger.Errorf("failed to delete kubeConfig: %v", err)
		return err
	}

	return nil
}

func (c *kubeConfigs) Get(ctx context.Context, kid int64) (*types.KubeConfig, error) {
	if c.client == nil {
		return nil, clientError
	}
	obj, err := c.factory.KubeConfig().Get(ctx, kid)
	if err != nil {
		log.Logger.Errorf("failed to get kubeConfig: %v", err)
		return nil, err
	}
	configByte, err := cipher.Decrypt(obj.Config)
	if err != nil {
		log.Logger.Errorf("failed to decrypt kubeConfig: %v", err)
		return nil, err
	}
	kubeConfig := c.model2Type(obj)
	kubeConfig.Config = string(configByte)

	return kubeConfig, nil
}

func (c *kubeConfigs) List(ctx context.Context, cloudName string) ([]types.KubeConfig, error) {
	if c.client == nil {
		return nil, clientError
	}
	var configs []types.KubeConfig
	objs, err := c.factory.KubeConfig().List(ctx, cloudName)
	if err != nil {
		log.Logger.Errorf("failed to list kubeConfig: %v", err)
		return nil, err
	}
	for _, obj := range objs {
		configByte, err := cipher.Decrypt(obj.Config)
		if err != nil {
			log.Logger.Errorf("failed to decrypt kubeConfig: %v", err)
			return nil, err
		}
		kubeConfig := c.model2Type(&obj)
		kubeConfig.Config = string(configByte)
		configs = append(configs, *kubeConfig)
	}

	return configs, nil
}

// createToken 默认token有效期一个月
func (c *kubeConfigs) createToken(ctx context.Context, saName string) (*authenticationv1.TokenRequest, error) {
	var expirationSeconds int64 = 2592000 // 1 month
	tokenRequest := &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{
			ExpirationSeconds: &expirationSeconds,
		},
	}
	token, err := c.client.CoreV1().ServiceAccounts(namespace).CreateToken(ctx, saName, tokenRequest, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return token, nil
}

func (c *kubeConfigs) model2Type(m *model.KubeConfig) *types.KubeConfig {
	return &types.KubeConfig{
		Id:                  m.Id,
		CloudName:           m.CloudName,
		ServiceAccount:      m.ServiceAccount,
		ClusterRole:         m.ClusterRole,
		Config:              m.Config,
		ExpirationTimestamp: m.ExpirationTimestamp,
	}
}

func (c *kubeConfigs) type2Model(t *types.KubeConfig) *model.KubeConfig {
	return &model.KubeConfig{
		CloudName:           t.CloudName,
		ServiceAccount:      t.ServiceAccount,
		ClusterRole:         t.ClusterRole,
		Config:              t.Config,
		ExpirationTimestamp: t.ExpirationTimestamp,
	}
}

type kubeconfig struct {
	APIVersion     string `yaml:"apiVersion"`
	Kind           string `yaml:"kind"`
	CurrentContext string `yaml:"current-context"`
	Clusters       []struct {
		Name    string `yaml:"name"`
		Cluster struct {
			Server                   string `yaml:"server"`
			CertificateAuthorityData string `yaml:"certificate-authority-data"`
		} `yaml:"cluster"`
	} `yaml:"clusters"`
	Contexts []struct {
		Name    string `yaml:"name"`
		Context struct {
			Cluster string `yaml:"cluster"`
			User    string `yaml:"user"`
		} `yaml:"context"`
	} `yaml:"contexts"`
	Users []struct {
		Name string `yaml:"name"`
		User struct {
			Token string `yaml:"token"`
		} `yaml:"user"`
	} `yaml:"users"`
}

func newConfig() *kubeconfig {
	return &kubeconfig{
		APIVersion:     "v1",
		Kind:           "Config",
		CurrentContext: "kubernetes",
		Clusters: []struct {
			Name    string `yaml:"name"`
			Cluster struct {
				Server                   string `yaml:"server"`
				CertificateAuthorityData string `yaml:"certificate-authority-data"`
			} `yaml:"cluster"`
		}{
			{
				Name: "kubernetes",
				Cluster: struct {
					Server                   string `yaml:"server"`
					CertificateAuthorityData string `yaml:"certificate-authority-data"`
				}{},
			},
		},
		Contexts: []struct {
			Name    string `yaml:"name"`
			Context struct {
				Cluster string `yaml:"cluster"`
				User    string `yaml:"user"`
			} `yaml:"context"`
		}{
			{
				Name: "kubernetes",
				Context: struct {
					Cluster string `yaml:"cluster"`
					User    string `yaml:"user"`
				}{
					Cluster: "kubernetes",
				},
			},
		},
		Users: []struct {
			Name string `yaml:"name"`
			User struct {
				Token string `yaml:"token"`
			} `yaml:"user"`
		}{
			{
				User: struct {
					Token string `yaml:"token"`
				}{},
			},
		},
	}
}

func (c *kubeconfig) setServer(server string) {
	c.Clusters[0].Cluster.Server = server
}

func (c *kubeconfig) setCA(ca string) {
	c.Clusters[0].Cluster.CertificateAuthorityData = ca
}

func (c *kubeconfig) setSA(saName, saToken string) {
	c.Contexts[0].Context.User = saName
	c.Users[0].Name = saName
	c.Users[0].User.Token = saToken
}
