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
	"bytes"
	"context"
	"fmt"
	"strconv"
	"time"

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
	pixiuerrors "github.com/caoyingjunz/gopixiu/pkg/errors"
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

// TODO: 移到pixiu自己的 ns 中
const namespace = "kube-system"

// TODO: cluster role 的优化指定
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
		return nil, pixiuerrors.ErrCloudNotRegister
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

	// 获取kubeConfig文件
	kubeConfigObj, err := c.factory.KubeConfig().GetByCloud(ctx, cloudObj.Id)
	if err != nil {
		return nil, err
	}
	// 解密集群 config
	cloudConfigByte, err := cipher.Decrypt(kubeConfigObj.Config)
	if err != nil {
		log.Logger.Errorf("failed to Decrypt cloud KubeConfig: %v", err)
		return nil, err
	}
	configData, err := Load(cloudConfigByte)
	if err != nil {
		log.Logger.Error("failed to load kubeConfig: %v", err)
		return nil, err
	}

	var (
		serverUrl string
		caCert    []byte
	)
	// TODO: 获取 caCert的方式需要更加严谨
	for _, cluster := range configData.Clusters {
		serverUrl = cluster.Server
		caCert = cluster.CertificateAuthorityData
		break
	}
	// TODO: 封装一个k8s的函数，包含 sa，roleBinding 和 token 的封装
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
	config := createConfigWithToken(caCert, kubeConfig.CloudName, serverUrl, kubeConfig.ServiceAccount, token.Status.Token)
	configBuf, err := ConfigMarshal(config)
	if err != nil {
		log.Logger.Error("failed to marshal kubeConfig: %v", err)
		return nil, err
	}

	kubeConfig.Config = configBuf.String()
	kubeConfig.ExpirationTimestamp = token.Status.ExpirationTimestamp.String()

	// 写库, kubeConfig 内容进行加密
	encryptConfig, err := cipher.Encrypt(configBuf.Bytes())
	if err != nil {
		log.Logger.Errorf("failed to encrypt kubeConfig: %v", err)
		return nil, err
	}
	obj, err := c.factory.KubeConfig().Create(ctx, &model.KubeConfig{
		CloudName:           kubeConfig.CloudName,
		ServiceAccount:      kubeConfig.ServiceAccount,
		ClusterRole:         kubeConfig.ClusterRole,
		Config:              encryptConfig,
		ExpirationTimestamp: kubeConfig.ExpirationTimestamp,
	})
	if err != nil {
		log.Logger.Errorf("failed to create kubeConfig: %v", err)
		return nil, err
	}
	kubeConfig.Id = obj.Id

	return kubeConfig, nil
}

func (c *kubeConfigs) Update(ctx context.Context, id int64) (*types.KubeConfigOptions, error) {
	if c.client == nil {
		return nil, pixiuerrors.ErrCloudNotRegister
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
	config, err := Load(configByte)
	if err != nil {
		log.Logger.Error("failed to load kubeConfig: %v", err)
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
	// 重建 token
	token, err := c.createToken(ctx, obj.ServiceAccount)
	if err != nil {
		log.Logger.Errorf("failed to get token: %v", err)
		return nil, err
	}
	setToken(config, obj.ServiceAccount, token.Status.Token)
	// 写库, kubeConfig 内容进行加密
	configBuf, err := ConfigMarshal(config)
	if err != nil {
		log.Logger.Error("failed to marshal kubeConfig: %v", err)
		return nil, err
	}
	encryptConfig, err := cipher.Encrypt(configBuf.Bytes())
	if err != nil {
		log.Logger.Errorf("failed to encrypt kubeConfig: %v", err)
		return nil, err
	}
	if err = c.factory.KubeConfig().Update(ctx, id, obj.ResourceVersion,
		map[string]interface{}{"config": encryptConfig},
	); err != nil {
		log.Logger.Errorf("failed to update kubeConfig: %v", err)
		return nil, err
	}

	kubeConfig := c.model2Type(obj)
	kubeConfig.Config = configBuf.String()
	kubeConfig.ExpirationTimestamp = token.Status.ExpirationTimestamp.String()

	return kubeConfig, nil
}

func (c *kubeConfigs) Delete(ctx context.Context, id int64) error {
	if c.client == nil {
		return pixiuerrors.ErrCloudNotRegister
	}

	obj, err := c.factory.KubeConfig().Get(ctx, id)
	if err != nil {
		return err
	}
	if err = c.factory.KubeConfig().Delete(ctx, id); err != nil {
		log.Logger.Errorf("failed to delete kubeConfig: %v", err)
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

	return nil
}

func (c *kubeConfigs) Get(ctx context.Context, id int64) (*types.KubeConfigOptions, error) {
	if c.client == nil {
		return nil, pixiuerrors.ErrCloudNotRegister
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
		return nil, pixiuerrors.ErrCloudNotRegister
	}

	objs, err := c.factory.KubeConfig().List(ctx, c.cloud)
	if err != nil {
		log.Logger.Errorf("failed to list kubeConfig: %v", err)
		return nil, err
	}

	var configs []types.KubeConfigOptions
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

func createConfigWithToken(caCert []byte, clusterName, serverURL, userName, token string) *clientcmdapi.Config {
	contextName := fmt.Sprintf("%s@%s", userName, clusterName)
	config := clientcmdapi.NewConfig()
	config.CurrentContext = contextName
	config.Clusters[clusterName] = &clientcmdapi.Cluster{
		Server:                   serverURL,
		CertificateAuthorityData: caCert,
	}
	config.Contexts[contextName] = &clientcmdapi.Context{
		Cluster:  clusterName,
		AuthInfo: userName,
	}
	config.AuthInfos[userName] = &clientcmdapi.AuthInfo{
		Token: token,
	}

	return config
}

func setToken(config *clientcmdapi.Config, userName, token string) {
	config.AuthInfos[userName].Token = token
}

func ConfigMarshal(config *clientcmdapi.Config) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	if err := clientcmdlatest.Codec.Encode(config, &buf); err != nil {
		return nil, err
	}
	return &buf, nil
}
