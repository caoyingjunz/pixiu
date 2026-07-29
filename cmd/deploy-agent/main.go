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

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"k8s.io/klog/v2"

	deployctl "github.com/caoyingjunz/pixiu/pkg/controller/deployagent"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

const version = "v0.1.0"

func main() {
	server := strings.TrimSpace(os.Getenv("PIXIU_SERVER"))
	token := strings.TrimSpace(os.Getenv("PIXIU_DEPLOY_TOKEN"))
	if server == "" || token == "" {
		klog.Fatalf("PIXIU_SERVER and PIXIU_DEPLOY_TOKEN are required")
	}
	server = strings.TrimRight(server, "/")
	workRoot := os.Getenv("PIXIU_AGENT_WORKDIR")
	if workRoot == "" {
		workRoot = "/var/lib/pixiu-deploy-agent"
	}
	_ = os.MkdirAll(workRoot, 0o755)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	hostname, _ := os.Hostname()
	api := &pixiuAPI{server: server, token: token, client: &http.Client{Timeout: 0}}

	klog.Infof("deploy-agent %s starting, server=%s", version, server)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = api.heartbeat(hostname)
			job, err := api.claim()
			if err != nil {
				klog.Errorf("claim failed: %v", err)
				continue
			}
			if job == nil {
				continue
			}
			klog.Infof("claimed job %d kind=%s action=%s", job.Id, job.Kind, job.Action)
			if err = runJob(ctx, api, workRoot, job); err != nil {
				klog.Errorf("job %d failed: %v", job.Id, err)
				_ = api.report(job.Id, false, err.Error(), "")
				continue
			}
		}
	}
}

type pixiuAPI struct {
	server string
	token  string
	client *http.Client
}

func (a *pixiuAPI) do(method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, a.server+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set(deployctl.TokenHeader, a.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("http %d: %s", resp.StatusCode, string(data))
	}
	if out == nil {
		return nil
	}
	var wrap struct {
		Result json.RawMessage `json:"result"`
	}
	if err = json.Unmarshal(data, &wrap); err != nil {
		return err
	}
	if len(wrap.Result) == 0 || string(wrap.Result) == "null" {
		return nil
	}
	return json.Unmarshal(wrap.Result, out)
}

func (a *pixiuAPI) heartbeat(hostname string) error {
	return a.do(http.MethodPost, "/pixiu/deploy-agents/heartbeat", types.DeployAgentHeartbeatRequest{
		Hostname: hostname,
		Version:  version,
	}, nil)
}

func (a *pixiuAPI) claim() (*types.DeployJob, error) {
	var job types.DeployJob
	if err := a.do(http.MethodGet, "/pixiu/deploy-agents/tasks/claim", nil, &job); err != nil {
		return nil, err
	}
	if job.Id == 0 {
		return nil, nil
	}
	return &job, nil
}

func (a *pixiuAPI) logs(jobId int64, chunk string) error {
	return a.do(http.MethodPost, fmt.Sprintf("/pixiu/deploy-agents/tasks/%d/logs", jobId),
		types.DeployJobLogsRequest{Chunk: chunk}, nil)
}

func (a *pixiuAPI) report(jobId int64, success bool, message, result string) error {
	return a.do(http.MethodPost, fmt.Sprintf("/pixiu/deploy-agents/tasks/%d/result", jobId),
		types.DeployJobResultRequest{Success: success, Message: message, Result: result}, nil)
}

func (a *pixiuAPI) downloadBundle(jobId int64, destDir string) error {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/pixiu/deploy-agents/tasks/%d/bundle", a.server, jobId), nil)
	if err != nil {
		return err
	}
	req.Header.Set(deployctl.TokenHeader, a.token)
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("bundle http %d: %s", resp.StatusCode, string(b))
	}
	return untarGz(resp.Body, destDir)
}

func runJob(ctx context.Context, api *pixiuAPI, workRoot string, job *types.DeployJob) error {
	switch job.Kind {
	case model.DeployJobPullImage:
		return pullImage(ctx, api, job)
	case model.DeployJobRunContainer:
		return runContainer(ctx, api, workRoot, job)
	case model.DeployJobFetchKubeconfig:
		return fetchKubeconfig(api, job)
	default:
		return fmt.Errorf("unknown job kind %s", job.Kind)
	}
}

func pullImage(ctx context.Context, api *pixiuAPI, job *types.DeployJob) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()
	_ = api.logs(job.Id, fmt.Sprintf("pulling image %s\n", job.Image))
	reader, err := cli.ImagePull(ctx, job.Image, image.PullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()
	_, _ = io.Copy(io.Discard, reader)
	return api.report(job.Id, true, "image ready", "")
}

func runContainer(ctx context.Context, api *pixiuAPI, workRoot string, job *types.DeployJob) error {
	planDir := filepath.Join(workRoot, fmt.Sprintf("%d", job.PlanId))
	_ = os.RemoveAll(planDir)
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		return err
	}
	if err := api.downloadBundle(job.Id, planDir); err != nil {
		return fmt.Errorf("download bundle: %w", err)
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()

	name := fmt.Sprintf("%s-%d", job.Action, job.PlanId)
	_ = removeContainerByName(ctx, cli, name)

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: job.Image,
		Env:   []string{fmt.Sprintf("COMMAND=%s", job.Action)},
	}, &container.HostConfig{
		Binds:       []string{fmt.Sprintf("%s:/configs", planDir)},
		NetworkMode: network.NetworkHost,
	}, nil, nil, name)
	if err != nil {
		return err
	}
	if err = cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return err
	}

	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err = <-errCh:
		if err != nil {
			return err
		}
	case st := <-statusCh:
		logs, _ := cli.ContainerLogs(ctx, resp.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true})
		if logs != nil {
			b, _ := io.ReadAll(logs)
			_ = logs.Close()
			_ = api.logs(job.Id, string(b))
		}
		if st.StatusCode != 0 {
			return fmt.Errorf("container exit code %d", st.StatusCode)
		}
	}
	return api.report(job.Id, true, "ok", "")
}

func removeContainerByName(ctx context.Context, cli *client.Client, name string) error {
	cs, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return err
	}
	for _, c := range cs {
		for _, n := range c.Names {
			if n == "/"+name || n == name {
				timeout := 5
				_ = cli.ContainerStop(ctx, c.ID, container.StopOptions{Timeout: &timeout})
				return cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true})
			}
		}
	}
	return nil
}

func fetchKubeconfig(api *pixiuAPI, job *types.DeployJob) error {
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
			_ = api.logs(job.Id, fmt.Sprintf("master %s: %v\n", m.Ip, err))
			continue
		}
		return api.report(job.Id, true, "ok", base64.StdEncoding.EncodeToString(cfg))
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
	sshClient, err := ssh.Dial("tcp", net.JoinHostPort(ip, "22"), &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec
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

func untarGz(r io.Reader, dest string) error {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dest, h.Name)
		switch h.Typeflag {
		case tar.TypeDir:
			if err = os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err = os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(h.Mode))
			if err != nil {
				return err
			}
			if _, err = io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}
}
