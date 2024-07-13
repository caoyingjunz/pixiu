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

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

const (
	KubeConfigFile = "/etc/kubernetes/admin.conf"
)

type Register struct {
	handlerTask

	factory db.ShareDaoFactory
}

func (c Register) Name() string { return "集群注册" }
func (c Register) Run() error {
	ks := &types.KubernetesSpec{}
	if err := ks.Unmarshal(c.data.Config.Kubernetes); err != nil {
		return err
	}
	// 如果未启用自注册功能，则直接跳过
	if !ks.Register {
		klog.Infof("部署计划未启用自注册功能，skipping")
	}

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
		return fmt.Errorf("get the empty kubeconfig from master nodes")
	}

	config64 := base64.StdEncoding.EncodeToString(kubeConfig)
	if err = c.factory.Cluster().UpdateByPlan(context.TODO(),
		c.data.PlanId, map[string]interface{}{"kube_config": config64}); err != nil {
		return err
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
