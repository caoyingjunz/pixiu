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

package ssh

import (
	"fmt"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/caoyingjunz/pixiu/pkg/types"
)

func NewSSHClient(sshConfig *types.WebSSHRequest) (*ssh.Client, error) {
	port := sshConfig.Port
	if port == 0 {
		port = 22
	}

	addr := fmt.Sprintf("%s:%d", sshConfig.Host, port)
	cfg := &ssh.ClientConfig{
		Timeout:         time.Second * 5,
		User:            sshConfig.User,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	if sshConfig.PrivateKey != "" {
		signer, err := ParsePrivateKeySigner(sshConfig.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		cfg.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	} else {
		cfg.Auth = []ssh.AuthMethod{ssh.Password(sshConfig.Password)}
	}

	return ssh.Dial("tcp", addr, cfg)
}
