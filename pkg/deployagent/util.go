/*
Copyright 2026 The Pixiu Authors.

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

package deployagent

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/caoyingjunz/pixiu/pkg/planrender"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

func renderConfig(ag *Agent, workRoot string, job *types.Job) error {
	_ = ag.Logs(job.Id, "fetching plan and rendering locally\n")
	if err := preparePlanDir(ag, workRoot, job); err != nil {
		return err
	}
	return ag.Report(job.Id, true, "render ok", "")
}

func preparePlanDir(ag *Agent, workRoot string, job *types.Job) error {
	plan, err := ag.FetchPlan(job.Id)
	if err != nil {
		return fmt.Errorf("fetch plan: %w", err)
	}
	planDir := filepath.Join(workRoot, fmt.Sprintf("%d", plan.Id))
	_ = os.RemoveAll(planDir)
	if err = os.MkdirAll(planDir, 0o755); err != nil {
		return err
	}
	if err = planrender.Render(workRoot, plan); err != nil {
		return fmt.Errorf("render: %w", err)
	}
	_ = ag.Logs(job.Id, fmt.Sprintf("rendered plan %d to %s\n", plan.Id, planDir))
	return nil
}

func fetchKubeconfig(ag *Agent, job *types.Job) error {
	var payload struct {
		Masters []struct {
			Name string `json:"name"`
			Ip   string `json:"ip"`
			Auth string `json:"auth"`
		} `json:"masters"`
	}
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return err
	}
	var lastErr error
	for _, m := range payload.Masters {
		cfg, err := sshGetAdminConf(m.Ip, m.Auth)
		if err != nil {
			lastErr = err
			_ = ag.Logs(job.Id, fmt.Sprintf("master %s: %v\n", m.Ip, err))
			continue
		}
		return ag.Report(job.Id, true, "ok", base64.StdEncoding.EncodeToString(cfg))
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no master nodes in payload")
	}
	return lastErr
}

func sshGetAdminConf(ip, authJSON string) ([]byte, error) {
	var auth types.PlanNodeAuth
	if err := auth.Unmarshal(authJSON); err != nil {
		return nil, err
	}
	var (
		user       string
		authMethod ssh.AuthMethod
	)
	switch auth.Type {
	case types.PasswordAuth:
		if auth.Password == nil {
			return nil, fmt.Errorf("password auth missing")
		}
		user = auth.Password.User
		authMethod = ssh.Password(auth.Password.Password)
	case types.KeyAuth:
		if auth.Key == nil {
			return nil, fmt.Errorf("key auth missing")
		}
		signer, err := ssh.ParsePrivateKey([]byte(auth.Key.Data))
		if err != nil {
			return nil, err
		}
		user = "root"
		authMethod = ssh.PublicKeys(signer)
	default:
		return nil, fmt.Errorf("unsupported auth type %v", auth.Type)
	}
	sshClient, err := ssh.Dial("tcp", net.JoinHostPort(ip, strconv.Itoa(auth.SSHPort())), &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	defer sshClient.Close()
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return nil, err
	}
	defer sftpClient.Close()
	f, err := sftpClient.Open("/etc/kubernetes/admin.conf")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}
