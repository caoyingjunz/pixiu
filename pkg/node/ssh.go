/*
Copyright 2024 The Pixiu Authors.

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

package node

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

func sshClientConfigPassword(config *types.WebSSHConfig) *model.SSHClientConfig {
	return &model.SSHClientConfig{
		Timeout:   time.Second * 5,
		AuthModel: model.PASSWORD,
		Host:      config.Host,
		Port:      config.Port,
		User:      config.User,
		Password:  config.Password,
		Protocol:  config.Protocol,
	}
}

func sshClientConfigPulicKey(config *types.WebSSHConfig) *model.SSHClientConfig {
	return &model.SSHClientConfig{
		Timeout:   time.Second * 5,
		AuthModel: model.PUBLICKEY,
		Host:      config.Host,
		Port:      config.Port,
		User:      config.User,
		KeyPath:   config.PkPath,
		Protocol:  config.Protocol,
	}
}

func newSSHClient(conf *model.SSHClientConfig) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		Timeout:         conf.Timeout,
		User:            conf.User,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //忽略know_hosts检查
	}

	switch conf.AuthModel {
	case model.PASSWORD:
		config.Auth = []ssh.AuthMethod{ssh.Password(conf.Password)}
	case model.PUBLICKEY:
		signer, err := getKey(conf.KeyPath)
		if err != nil {
			return nil, err
		}

		config.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	}
	c, err := ssh.Dial(conf.Protocol, fmt.Sprintf("%s:%d", conf.Host, conf.Port), config)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func getKey(keyPath string) (ssh.Signer, error) {
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	return ssh.ParsePrivateKey(key)
}
