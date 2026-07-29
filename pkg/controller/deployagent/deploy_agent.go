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
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/caoyingjunz/pixiu/pkg/util/archive"
)

const TokenHeader = "X-Pixiu-Deploy-Token"

type Getter interface {
	DeployAgent() Interface
}

type Interface interface {
	// Admin
	Create(ctx context.Context, req *types.CreateDeployAgentRequest) (*types.DeployAgentCreateResponse, error)
	List(ctx context.Context) ([]types.DeployAgent, error)
	Get(ctx context.Context, id int64) (*types.DeployAgent, error)
	Delete(ctx context.Context, id int64) error
	GetInstall(ctx context.Context, id int64) (*types.DeployAgentInstallResponse, error)

	// Agent HTTPS job APIs (token auth)
	Heartbeat(ctx context.Context, token string, req *types.DeployAgentHeartbeatRequest) error
	Claim(ctx context.Context, token string) (*types.DeployJob, error)
	AppendLogs(ctx context.Context, token string, jobId int64, req *types.DeployJobLogsRequest) error
	ReportResult(ctx context.Context, token string, jobId int64, req *types.DeployJobResultRequest) error
	BundlePath(ctx context.Context, token string, jobId int64) (string, error)
}

type controller struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &controller{cc: cfg, factory: f}
}

func (c *controller) Create(ctx context.Context, req *types.CreateDeployAgentRequest) (*types.DeployAgentCreateResponse, error) {
	token, err := generateToken()
	if err != nil {
		return nil, errors.ErrServerInternal
	}
	obj, err := c.factory.DeployAgent().Create(ctx, &model.DeployAgent{
		Name:        req.Name,
		Token:       token,
		Status:      model.DeployAgentStatusOffline,
		Description: req.Description,
	})
	if err != nil {
		klog.Errorf("create deploy agent failed: %v", err)
		return nil, errors.ErrServerInternal
	}
	return &types.DeployAgentCreateResponse{
		DeployAgent: toType(obj),
		Token:       token,
	}, nil
}

func (c *controller) List(ctx context.Context) ([]types.DeployAgent, error) {
	objects, err := c.factory.DeployAgent().List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]types.DeployAgent, 0, len(objects))
	for i := range objects {
		out = append(out, toType(&objects[i]))
	}
	return out, nil
}

func (c *controller) Get(ctx context.Context, id int64) (*types.DeployAgent, error) {
	obj, err := c.factory.DeployAgent().Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, errors.NewError(fmt.Errorf("deploy agent not found"), http.StatusNotFound)
	}
	t := toType(obj)
	return &t, nil
}

func (c *controller) Delete(ctx context.Context, id int64) error {
	return c.factory.DeployAgent().Delete(ctx, id)
}

func (c *controller) GetInstall(ctx context.Context, id int64) (*types.DeployAgentInstallResponse, error) {
	obj, err := c.factory.DeployAgent().Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, errors.NewError(fmt.Errorf("deploy agent not found"), http.StatusNotFound)
	}
	serverURL := strings.TrimRight(c.cc.Default.PublicURL, "/")
	if serverURL == "" {
		serverURL = "https://<pixiu-server>"
	}
	return &types.DeployAgentInstallResponse{
		ServerURL: serverURL,
		Token:     obj.Token,
		Command: fmt.Sprintf(
			"PIXIU_SERVER=%s PIXIU_DEPLOY_TOKEN=%s deploy-agent",
			serverURL, obj.Token,
		),
	}, nil
}

func (c *controller) authAgent(ctx context.Context, token string) (*model.DeployAgent, error) {
	obj, err := c.factory.DeployAgent().GetByToken(ctx, strings.TrimSpace(token))
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, errors.NewError(fmt.Errorf("invalid deploy agent token"), http.StatusUnauthorized)
	}
	return obj, nil
}

func (c *controller) Heartbeat(ctx context.Context, token string, req *types.DeployAgentHeartbeatRequest) error {
	obj, err := c.authAgent(ctx, token)
	if err != nil {
		return err
	}
	updates := map[string]interface{}{
		"status":         model.DeployAgentStatusOnline,
		"last_heartbeat": time.Now(),
	}
	if req != nil {
		if req.Hostname != "" {
			updates["hostname"] = req.Hostname
		}
		if req.Version != "" {
			updates["version"] = req.Version
		}
	}
	return c.factory.DeployAgent().InternalUpdate(ctx, obj.Id, updates)
}

func (c *controller) Claim(ctx context.Context, token string) (*types.DeployJob, error) {
	obj, err := c.authAgent(ctx, token)
	if err != nil {
		return nil, err
	}
	_ = c.factory.DeployAgent().InternalUpdate(ctx, obj.Id, map[string]interface{}{
		"status":         model.DeployAgentStatusOnline,
		"last_heartbeat": time.Now(),
	})
	job, err := c.factory.DeployJob().ClaimNext(ctx, obj.Id)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, nil
	}
	t := jobToType(job)
	return &t, nil
}

func (c *controller) AppendLogs(ctx context.Context, token string, jobId int64, req *types.DeployJobLogsRequest) error {
	obj, err := c.authAgent(ctx, token)
	if err != nil {
		return err
	}
	job, err := c.factory.DeployJob().Get(ctx, jobId)
	if err != nil {
		return err
	}
	if job == nil || job.AgentId != obj.Id {
		return errors.NewError(fmt.Errorf("job not found"), http.StatusNotFound)
	}
	if req == nil {
		return nil
	}
	return c.factory.DeployJob().AppendLogs(ctx, jobId, req.Chunk)
}

func (c *controller) ReportResult(ctx context.Context, token string, jobId int64, req *types.DeployJobResultRequest) error {
	obj, err := c.authAgent(ctx, token)
	if err != nil {
		return err
	}
	job, err := c.factory.DeployJob().Get(ctx, jobId)
	if err != nil {
		return err
	}
	if job == nil || job.AgentId != obj.Id {
		return errors.NewError(fmt.Errorf("job not found"), http.StatusNotFound)
	}
	if req == nil {
		return errors.NewError(fmt.Errorf("empty result"), http.StatusBadRequest)
	}
	status := model.DeployJobSuccess
	if !req.Success {
		status = model.DeployJobFailed
	}
	return c.factory.DeployJob().InternalUpdate(ctx, jobId, map[string]interface{}{
		"status":  status,
		"message": req.Message,
		"result":  req.Result,
	})
}

func (c *controller) BundlePath(ctx context.Context, token string, jobId int64) (string, error) {
	obj, err := c.authAgent(ctx, token)
	if err != nil {
		return "", err
	}
	job, err := c.factory.DeployJob().Get(ctx, jobId)
	if err != nil {
		return "", err
	}
	if job == nil || job.AgentId != obj.Id {
		return "", errors.NewError(fmt.Errorf("job not found"), http.StatusNotFound)
	}
	workDir := c.cc.Worker.WorkDir
	if workDir == "" {
		workDir = "/etc/pixiu"
	}
	src := filepath.Join(workDir, fmt.Sprintf("%d", job.PlanId))
	if _, err = os.Stat(src); err != nil {
		return "", errors.NewError(fmt.Errorf("plan bundle not ready: %v", err), http.StatusNotFound)
	}
	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("pixiu-plan-%d-%d.tar.gz", job.PlanId, time.Now().UnixNano()))
	if err = archive.TarGzDir(src, tmp); err != nil {
		return "", err
	}
	return tmp, nil
}

func generateToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func toType(o *model.DeployAgent) types.DeployAgent {
	return types.DeployAgent{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
		Name:          o.Name,
		Status:        o.Status,
		Hostname:      o.Hostname,
		Version:       o.Version,
		LastHeartbeat: o.LastHeartbeat,
		Description:   o.Description,
	}
}

func jobToType(o *model.DeployJob) types.DeployJob {
	return types.DeployJob{
		Id:       o.Id,
		PlanId:   o.PlanId,
		AgentId:  o.AgentId,
		TaskName: o.TaskName,
		Kind:     o.Kind,
		Action:   o.Action,
		Image:    o.Image,
		Payload:  o.Payload,
		Status:   o.Status,
		Message:  o.Message,
	}
}
