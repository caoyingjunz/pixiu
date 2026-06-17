/*
Copyright 2021 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (phe "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package plan

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/caoyingjunz/pixiu/pkg/client"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/caoyingjunz/pixiu/pkg/util/uuid"
)

const (
	KubeConfigFile = "/etc/kubernetes/admin.conf"
)

type Register struct {
	handlerTask

	factory db.ShareDaoFactory
}

func (c Register) Name() string      { return "集群注册" }
func (c Register) GetAction() string { return "register" }
func (c Register) Run() error {
	ks := &types.KubernetesSpec{}
	if err := ks.Unmarshal(c.data.Config.Kubernetes); err != nil {
		return err
	}
	// 如果未启用自注册功能，则直接跳过
	// if !ks.Register {
	// 	klog.Infof("部署计划未启用自注册功能，skipping")
	// 	return nil
	// }

	// 从 master 节点获取 kubeConfig 内容，注入集群服务
	var masterNodes []model.Node
	for _, node := range c.data.Nodes {
		if strings.Contains(node.Role, model.MasterRole) {
			masterNodes = append(masterNodes, node)
		}
	}

	var (
		kubeConfig []byte
		err        error
	)
	for _, masterNode := range masterNodes {
		kubeConfig, err = getKubeConfigFromMasterNode(masterNode)
		if err == nil {
			break
		} else {
			klog.Warningf("failed to get kubeConfig from master(%s): %v, trying the other masters", masterNode.Name, err)
		}
	}
	if len(kubeConfig) == 0 {
		klog.Error("get the empty kubeconfig from master nodes")
		return fmt.Errorf("get the empty kubeconfig from master nodes")
	}
	config64 := base64.StdEncoding.EncodeToString(kubeConfig)

	// 1. 创建/更新集群
	// 检查plan对应的集群是否存在，如果已经存在则直接更新，不存在则新建
	objs, err := c.factory.Cluster().List(context.TODO(), db.WithUser(c.data.Plan.UserId), db.WithPlan(c.data.PlanId))
	if err != nil {
		return fmt.Errorf("get clusters error: %v", err)
	}
	if len(objs) == 0 {
		kubeNode := types.KubeNode{Ready: []string{}, NotReady: []string{}}
		nodes, _ := kubeNode.Marshal()
		_, err = c.factory.Cluster().Create(context.TODO(), &model.Cluster{
			Name:          uuid.NewRandName(8),
			AliasName:     c.data.Plan.Name,
			ClusterType:   model.ClusterTypeCustom,
			PlanId:        c.data.PlanId,
			UserId:        c.data.Plan.UserId,
			ClusterStatus: model.ClusterStatusDeploy,
			Protected:     true,
			Nodes:         nodes,
			KubeConfig:    config64,
		})
		if err != nil {
			klog.Errorf("failed to register cluster for plan: %v", err)
			return fmt.Errorf("failed to create cluster for plan %v", err)
		}
	} else {
		if err = c.factory.Cluster().UpdateByPlan(context.TODO(),
			c.data.PlanId, map[string]interface{}{"kube_config": config64}); err != nil {
			return err
		}
	}

	// 2. 注册权限规则
	if err = c.addPixiuClusterRole(context.TODO(), config64); err != nil {
		klog.Errorf("failed to add pixiu cluster role for plan register: %v", err)
		return err
	}
	return nil
}

// 创建内置的 ClusterRole
func (c Register) addPixiuClusterRole(ctx context.Context, kubeconfig string) error {
	cs, err := client.NewClusterSet(kubeconfig)
	if err != nil {
		return nil
	}

	clusterRoleView := "pixiu-view"
	// 已存在则忽略
	_, err = cs.Client.RbacV1().ClusterRoles().Get(ctx, clusterRoleView, metav1.GetOptions{})
	if err == nil {
		return nil
	}

	clusterRoleSystemView := "view"
	viewClusterRole, err := cs.Client.RbacV1().ClusterRoles().Get(ctx, clusterRoleSystemView, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("获取内置 ClusterRoles 失败 %v", err)
	}

	rules := viewClusterRole.Rules
	// 追加依赖rule
	rules = append(rules, []rbacv1.PolicyRule{
		{
			APIGroups: []string{"metrics.pixiu.io"},
			Resources: []string{"api/dashboard"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"nodes"},
			Verbs:     []string{"get", "list", "watch"},
		},
	}...)

	_, err = cs.Client.RbacV1().ClusterRoles().Create(ctx, &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleView,
			Labels: map[string]string{
				"maintainer": "pixiu",
			},
		},
		Rules: rules,
	}, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("创建你内置 ClusterRole 失败: %v", err)
	}
	return nil
}

func getKubeConfigFromMasterNode(maserNode model.Node) ([]byte, error) {
	sftpClient, err := newSftpClient(maserNode)
	if err != nil {
		return nil, err
	}
	defer sftpClient.Close()

	srcFile, err := sftpClient.Open(KubeConfigFile)
	if err != nil {
		return nil, err
	}
	defer srcFile.Close()

	buf, err := io.ReadAll(srcFile)
	if err != nil {
		return nil, err
	}

	return buf, nil
}

func newSftpClient(node model.Node) (*sftp.Client, error) {
	nodeAuth := types.PlanNodeAuth{}
	if err := nodeAuth.Unmarshal(node.Auth); err != nil {
		return nil, err
	}

	var clientConfig *ssh.ClientConfig

	switch nodeAuth.Type {
	case types.PasswordAuth:
		// 1. 使用密码
		clientConfig = &ssh.ClientConfig{
			User: nodeAuth.Password.User,
			Auth: []ssh.AuthMethod{
				ssh.Password(nodeAuth.Password.Password),
			},
			Timeout: 30 * time.Second,
			HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
				return nil
			},
		}
	case types.KeyAuth:
		//2. 使用秘钥
		key := []byte(nodeAuth.Key.Data)
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, err
		}
		clientConfig = &ssh.ClientConfig{
			User: "root", // 秘钥登陆时，默认 root
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(signer),
			},
			Timeout:         30 * time.Second,
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
	default:
		return nil, fmt.Errorf("unsupported ssh auth type: %s", nodeAuth.Type)
	}

	addr := fmt.Sprintf("%s:%d", node.Ip, 22)
	sshClient, err := ssh.Dial("tcp", addr, clientConfig)
	if err != nil {
		return nil, err
	}

	return sftp.NewClient(sshClient)
}
