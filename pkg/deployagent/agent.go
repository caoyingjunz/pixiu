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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/controller/agent"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

const Version = "v0.1.0"

// Agent 封装与 Pixiu 服务端的通信。
type Agent struct {
	server string
	token  string

	client *http.Client
}

func New(server, token string) *Agent {
	return &Agent{
		server: server,
		token:  token,
		client: &http.Client{Timeout: 5 * time.Minute},
	}
}

func (a *Agent) do(method, path string, body any, out any) error {
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
	req.Header.Set(agent.TokenHeader, a.token)
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

// Heartbeat 向服务端上报心跳。
func (a *Agent) Heartbeat(hostname string) error {
	klog.V(2).Infof("sending heartbeat to %s", a.server)
	return a.do(http.MethodPost, "/pixiu/agents/heartbeat", types.AgentHeartbeatRequest{Hostname: hostname, Version: Version}, nil)
}

// Claim 从服务端领取待执行作业。
func (a *Agent) Claim() (*types.Job, error) {
	var job types.Job
	if err := a.do(http.MethodGet, "/pixiu/agents/claim", nil, &job); err != nil {
		return nil, err
	}
	if job.Id == 0 {
		return nil, nil
	}
	return &job, nil
}

// Logs 向服务端追加作业日志。
func (a *Agent) Logs(jobId int64, chunk string) error {
	return a.do(http.MethodPost, fmt.Sprintf("/pixiu/agents/jobs/%d/logs", jobId),
		types.AgentJobLogsRequest{Chunk: chunk}, nil)
}

// Report 向服务端上报作业执行结果。
func (a *Agent) Report(jobId int64, success bool, message, result string) error {
	return a.do(http.MethodPost, fmt.Sprintf("/pixiu/agents/jobs/%d/result", jobId),
		types.AgentJobResultRequest{Success: success, Message: message, Result: result}, nil)
}

// FetchPlan 从服务端拉取部署计划数据。
func (a *Agent) FetchPlan(jobId int64) (*types.Plan, error) {
	var plan types.Plan
	if err := a.do(http.MethodGet, fmt.Sprintf("/pixiu/agents/jobs/%d/plan", jobId), nil, &plan); err != nil {
		return nil, err
	}
	if plan.Id == 0 {
		return nil, fmt.Errorf("empty plan")
	}
	return &plan, nil
}

// RunJob 根据作业类型分发到对应的执行器。
func RunJob(ctx context.Context, ag *Agent, workRoot string, job *types.Job) error {
	switch job.Kind {
	case model.JobPullImage:
		return pullImage(ctx, ag, job)
	case model.JobRenderConfig:
		return renderConfig(ag, workRoot, job)
	case model.JobRunContainer:
		return runContainer(ctx, ag, workRoot, job)
	case model.JobFetchKubeconfig:
		return fetchKubeconfig(ag, job)
	default:
		return fmt.Errorf("unknown job kind %s", job.Kind)
	}
}
