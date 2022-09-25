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

package kubernetes

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"time"

	"gopkg.in/yaml.v2"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"

	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/db"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	"github.com/caoyingjunz/gopixiu/pkg/log"
	"github.com/caoyingjunz/gopixiu/pkg/util/cipher"
)

type KubeConfigGetter interface {
	KubeConfigs(cloud string) KubeConfigInterface
}

type KubeConfigInterface interface {
	Create(ctx context.Context, opts *types.KubeConfigOptions) (*types.KubeConfigOptions, error)
	Update(ctx context.Context, id int64) (*types.KubeConfigOptions, error)
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (*types.KubeConfigOptions, error)
	List(ctx context.Context) ([]types.KubeConfigOptions, error)
}

type kubeConfigs struct {
	client  *kubernetes.Clientset
	cloud   string
	factory db.ShareDaoFactory
}

func NewKubeConfigs(c *kubernetes.Clientset, cloud string, factory db.ShareDaoFactory) KubeConfigInterface {
	return &kubeConfigs{
		client:  c,
		cloud:   cloud,
		factory: factory,
	}
}

const namespace = "kube-system"

// preCreate 创建前检查, 默认权限为 cluster-admin
func (c *kubeConfigs) preCreate(ctx context.Context, kubeConfig *types.KubeConfigOptions) error {
	if len(kubeConfig.ServiceAccount) == 0 {
		kubeConfig.ServiceAccount = strconv.FormatInt(time.Now().Unix(), 10)
	}
	// TODO: clusterRole 后端根据需要创建
	if len(kubeConfig.ClusterRole) == 0 {
		kubeConfig.ClusterRole = "cluster-admin"
	}

	return nil
}

func (c *kubeConfigs) Create(ctx context.Context, kubeConfig *types.KubeConfigOptions) (*types.KubeConfigOptions, error) {
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
	kubcConfig, err := Load(cloudConfigByte)
	if err != nil {
		return nil, err
	}

	var (
		serverUrl string
		caCert    string
	)
	for _, cluster := range kubcConfig.Clusters {
		serverUrl = cluster.Server
		caCert = base64.StdEncoding.EncodeToString(cluster.CertificateAuthorityData)
		break
	}

	// 创建 service account
	if err = c.createServiceAccount(ctx, kubeConfig.ServiceAccount); err != nil {
		log.Logger.Errorf("failed to create service account: %v", err)
		return nil, err
	}
	// 创建 cluster role binding
	if err = c.createClusterRoleBinding(ctx, kubeConfig.ServiceAccount, kubeConfig.ClusterRole); err != nil {
		log.Logger.Errorf("failed to create cluster role binding: %v", err)
		return nil, err
	}

	// 生成token
	token, err := c.createToken(ctx, kubeConfig.ServiceAccount)
	if err != nil {
		log.Logger.Errorf("failed to get token: %v", err)
		return nil, err
	}
	// 生成 config
	config := createKubeConfigWithToken(kubeConfig.CloudName, serverUrl, caCert, kubeConfig.ServiceAccount, token.Status.Token)

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

func (c *kubeConfigs) Update(ctx context.Context, id int64) (*types.KubeConfigOptions, error) {
	if c.client == nil {
		return nil, clientError
	}
	obj, err := c.factory.KubeConfig().Get(ctx, id)
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
	config := newKubeConfig()
	if err = yaml.Unmarshal(configByte, config); err != nil {
		log.Logger.Error(err)
		return nil, err
	}
	// 重建 service account
	if err = c.client.CoreV1().ServiceAccounts(namespace).Delete(ctx, obj.ServiceAccount, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		log.Logger.Errorf("failed to delete service account: %v", err)
	}
	if err = c.createServiceAccount(ctx, obj.ServiceAccount); err != nil {
		log.Logger.Errorf("failed to create service account: %v", err)
		return nil, err
	}
	// 创建 token
	token, err := c.createToken(ctx, obj.ServiceAccount)
	if err != nil {
		log.Logger.Errorf("failed to get token: %v", err)
		return nil, err
	}
	config.refreshToken(token.Status.Token)
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
	if err = c.factory.KubeConfig().Update(ctx, id, obj.ResourceVersion,
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

func (c *kubeConfigs) Delete(ctx context.Context, id int64) error {
	if c.client == nil {
		return clientError
	}
	obj, err := c.factory.KubeConfig().Get(ctx, id)
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
	if err = c.factory.KubeConfig().Delete(ctx, id); err != nil {
		log.Logger.Errorf("failed to delete kubeConfig: %v", err)
		return err
	}

	return nil
}

func (c *kubeConfigs) Get(ctx context.Context, id int64) (*types.KubeConfigOptions, error) {
	if c.client == nil {
		return nil, clientError
	}
	obj, err := c.factory.KubeConfig().Get(ctx, id)
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

func (c *kubeConfigs) List(ctx context.Context) ([]types.KubeConfigOptions, error) {
	if c.client == nil {
		return nil, clientError
	}
	var configs []types.KubeConfigOptions
	objs, err := c.factory.KubeConfig().List(ctx, c.cloud)
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

func (c *kubeConfigs) createServiceAccount(ctx context.Context, saName string) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: saName,
		},
	}
	if _, err := c.client.CoreV1().ServiceAccounts(namespace).Create(ctx, sa, metav1.CreateOptions{}); err != nil {
		return err
	}

	return nil
}

func (c *kubeConfigs) createClusterRoleBinding(ctx context.Context, saName, clusterRoleName string) error {
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
			Name: clusterRoleName,
		},
	}
	if _, err := c.client.RbacV1().ClusterRoleBindings().Create(ctx, clusterRoleBinding, metav1.CreateOptions{}); err != nil {
		return err
	}

	return nil
}

func (c *kubeConfigs) model2Type(m *model.KubeConfig) *types.KubeConfigOptions {
	return &types.KubeConfigOptions{
		Id:                  m.Id,
		CloudName:           m.CloudName,
		ServiceAccount:      m.ServiceAccount,
		ClusterRole:         m.ClusterRole,
		Config:              m.Config,
		ExpirationTimestamp: m.ExpirationTimestamp,
	}
}

func (c *kubeConfigs) type2Model(t *types.KubeConfigOptions) *model.KubeConfig {
	return &model.KubeConfig{
		CloudName:           t.CloudName,
		ServiceAccount:      t.ServiceAccount,
		ClusterRole:         t.ClusterRole,
		Config:              t.Config,
		ExpirationTimestamp: t.ExpirationTimestamp,
	}
}

type Config struct {
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

func newKubeConfig() *Config {
	return &Config{
		APIVersion: "v1",
		Kind:       "Config",
		Clusters: []struct {
			Name    string `yaml:"name"`
			Cluster struct {
				Server                   string `yaml:"server"`
				CertificateAuthorityData string `yaml:"certificate-authority-data"`
			} `yaml:"cluster"`
		}{
			{
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
				Context: struct {
					Cluster string `yaml:"cluster"`
					User    string `yaml:"user"`
				}{},
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

func createKubeConfigWithToken(clusterName, serverURL, caCert, userName, token string) *Config {
	contextName := fmt.Sprintf("%s@%s", userName, clusterName)
	config := newKubeConfig()
	config.CurrentContext = contextName
	config.Clusters[0].Name = clusterName
	config.Clusters[0].Cluster.Server = serverURL
	config.Clusters[0].Cluster.CertificateAuthorityData = caCert
	config.Contexts[0].Name = contextName
	config.Contexts[0].Context.Cluster = clusterName
	config.Contexts[0].Context.User = userName
	config.Users[0].Name = userName
	config.Users[0].User.Token = token
	return config
}

func (c *Config) refreshToken(token string) {
	c.Users[0].User.Token = token
}

func Load(data []byte) (*clientcmdapi.Config, error) {
	config := clientcmdapi.NewConfig()
	// if there's no data in a file, return the default object instead of failing (DecodeInto reject empty input)
	if len(data) == 0 {
		return config, nil
	}
	decoded, _, err := clientcmdlatest.Codec.Decode(data, &schema.GroupVersionKind{Version: clientcmdlatest.Version, Kind: "Config"}, config)
	if err != nil {
		return nil, err
	}
	return decoded.(*clientcmdapi.Config), nil
}
