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
	"encoding/base64"
	"fmt"
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
	for _, maserNode := range masterNodes {
		kubeConfig, err = getKubeConfigFromMasterNode(maserNode)
		if err == nil {
			break
		}
	}
	if len(kubeConfig) == 0 {
		return fmt.Errorf("failed to get kubeconfig from master node")
	}

	configBase64 := base64.StdEncoding.EncodeToString(kubeConfig)
	planId := c.data.PlanId

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

	buf := make([]byte, 1<<15)
	if _, err = srcFile.Read(buf); err != nil {
		return nil, err
	}

	return buf, nil
}

func newSftpClient(node model.Node) (*sftp.Client, error) {
	nodeAuth := types.PlanNodeAuth{}
	if err := nodeAuth.Unmarshal(node.Auth); err != nil {
		return nil, err
	}

	// 1. 使用密码
	clientConfig := &ssh.ClientConfig{
		User: nodeAuth.Password.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(nodeAuth.Password.Password),
		},
		Timeout: 30 * time.Second,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}

	// 2. 使用公钥
	//key, err := ioutil.ReadFile(path.Join(homedir.HomeDir(), ".ssh", "id_rsa"))
	//if err != nil {
	//	return nil, err
	//}
	//signer, err := ssh.ParsePrivateKey(key)
	//if err != nil {
	//	return nil, err
	//}
	//clientConfig := &ssh.ClientConfig{
	//	User: "root",
	//	Auth: []ssh.AuthMethod{
	//		ssh.PublicKeys(signer),
	//	},
	//	Timeout:         30 * time.Second,
	//	HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	//}

	addr := fmt.Sprintf("%s:%d", node.Ip, 22)
	sshClient, err := ssh.Dial("tcp", addr, clientConfig)
	if err != nil {
		return nil, err
	}

	return sftp.NewClient(sshClient)
}
