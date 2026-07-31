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

package plan

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

const (
	defaultJobTimeout = 60 * time.Minute
	jobPollInterval   = 2 * time.Second
)

// AgentJob 将部署步骤下发为 Job，等待边缘 Agent 执行完成。
type AgentJob struct {
	handlerTask

	factory   db.ShareDaoFactory
	agentId   int64
	stepName  string
	step      model.PlanStep
	kind      model.JobKind
	action    string
	image     string
	payload   string
	timeout   time.Duration
	onSuccess func(result string) error
}

func (a AgentJob) Name() string         { return a.stepName }
func (a AgentJob) GetAction() string    { return a.action }
func (a AgentJob) Step() model.PlanStep { return a.step }

func (a AgentJob) Run() error {
	ctx := context.Background()
	job, err := a.factory.Agent().Job().Create(ctx, &model.Job{
		PlanId:   a.GetPlanId(),
		AgentId:  a.agentId,
		TaskName: a.stepName,
		Kind:     a.kind,
		Action:   a.action,
		Image:    a.image,
		Payload:  a.payload,
		Status:   model.JobPending,
	})
	if err != nil {
		return fmt.Errorf("create deploy job: %w", err)
	}
	klog.Infof("created deploy job %d for plan(%d) task(%s)", job.Id, a.GetPlanId(), a.stepName)

	timeout := a.timeout
	if timeout <= 0 {
		timeout = defaultJobTimeout
	}
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			_ = a.factory.Agent().Job().InternalUpdate(ctx, job.Id, map[string]interface{}{
				"status":  model.JobFailed,
				"message": "wait agent timeout",
			})
			return fmt.Errorf("wait deploy job %d timeout", job.Id)
		}
		cur, err := a.factory.Agent().Job().Get(ctx, job.Id)
		if err != nil {
			return err
		}
		if cur == nil {
			return fmt.Errorf("deploy job %d disappeared", job.Id)
		}
		switch cur.Status {
		case model.JobSuccess:
			if a.onSuccess != nil {
				return a.onSuccess(cur.Result)
			}
			return nil
		case model.JobFailed:
			if cur.Message != "" {
				return fmt.Errorf("%s", cur.Message)
			}
			return fmt.Errorf("deploy job %d failed", job.Id)
		}
		time.Sleep(jobPollInterval)
	}
}

func buildRegisterPayload(nodes []model.Node) (string, error) {
	type nodeAuth struct {
		Name string `json:"name"`
		Ip   string `json:"ip"`
		Role string `json:"role"`
		Auth string `json:"auth"`
	}
	var masters []nodeAuth
	for _, n := range nodes {
		if strings.Contains(n.Role, model.MasterRole) {
			masters = append(masters, nodeAuth{Name: n.Name, Ip: n.Ip, Role: n.Role, Auth: n.Auth})
		}
	}
	b, err := json.Marshal(map[string]interface{}{"masters": masters})
	if err != nil {
		return "", err
	}
	return string(b), nil
}
