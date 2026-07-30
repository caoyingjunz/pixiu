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

package agent

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/caoyingjunz/pixiu/pkg/util/archive"
	"github.com/caoyingjunz/pixiu/pkg/util/token"
)

const TokenHeader = "X-Pixiu-Deploy-Token"

type AgentGetter interface {
	Agent() Interface
}

type Interface interface {
	Create(ctx context.Context, req *types.CreateAgentRequest) error
	Update(ctx context.Context, agentId int64, req *types.UpdateAgentRequest) error
	Delete(ctx context.Context, agentId int64) error
	Get(ctx context.Context, agentId int64) (*types.Agent, error)
	List(ctx context.Context, listOption types.ListOptions) (interface{}, error)

	Heartbeat(ctx context.Context, agentToken string, req *types.AgentHeartbeatRequest) error
	Claim(ctx context.Context, agentToken string) (*types.Job, error)

	AppendLogs(ctx context.Context, agentToken string, jobId int64, req *types.AgentJobLogsRequest) error
	ReportResult(ctx context.Context, agentToken string, jobId int64, req *types.AgentJobResultRequest) error
	BundlePath(ctx context.Context, agentToken string, jobId int64) (string, error)
}

type agentController struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func NewAgent(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &agentController{cc: cfg, factory: f}
}

func (a *agentController) Create(ctx context.Context, req *types.CreateAgentRequest) error {
	tkn, err := token.Generate()
	if err != nil {
		return errors.ErrServerInternal
	}
	userId, _ := httputils.GetUserIdFromContext(ctx)
	_, err = a.factory.Agent().Create(ctx, &model.Agent{
		Name:        req.Name,
		AgentType:   req.Type,
		UserID:      userId,
		Token:       tkn,
		Status:      model.AgentStatusOffline,
		Description: req.Description,
	})
	if err != nil {
		klog.Errorf("failed to create deploy agent %s: %v", req.Name, err)
		return errors.ErrServerInternal
	}
	return nil
}

func (a *agentController) Update(ctx context.Context, agentId int64, req *types.UpdateAgentRequest) error {
	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Type != nil {
		updates["agent_type"] = *req.Type
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if err := a.factory.Agent().Update(ctx, agentId, req.ResourceVersion, updates); err != nil {
		klog.Errorf("failed to update deploy agent %d: %v", agentId, err)
		return errors.ErrServerInternal
	}
	return nil
}

func (a *agentController) Delete(ctx context.Context, agentId int64) error {
	if err := a.factory.Agent().Delete(ctx, agentId); err != nil {
		klog.Errorf("failed to delete deploy agent %d: %v", agentId, err)
		return errors.ErrServerInternal
	}
	return nil
}

func (a *agentController) Get(ctx context.Context, agentId int64) (*types.Agent, error) {
	obj, err := a.factory.Agent().Get(ctx, agentId)
	if err != nil {
		klog.Errorf("failed to get deploy agent(%d): %v", agentId, err)
		return nil, errors.ErrServerInternal
	}
	if obj == nil {
		return nil, errors.ErrAgentNotFound
	}
	return toAgent(obj), nil
}

func (a *agentController) List(ctx context.Context, listOption types.ListOptions) (interface{}, error) {
	listOption.SetDefaultPageOption()
	pr := types.PageResult{PageRequest: types.PageRequest{Page: listOption.Page, Limit: listOption.Limit}}

	filters := buildAgentFilters(listOption)
	pr.Total, _ = a.factory.Agent().Count(ctx, filters...)
	objects, err := a.factory.Agent().List(ctx, append(filters,
		db.WithOffset((listOption.Page-1)*listOption.Limit),
		db.WithLimit(listOption.Limit),
		db.WithModifyOrderByDesc())...)
	if err != nil {
		return nil, err
	}
	items := make([]types.Agent, len(objects))
	for i := range objects {
		items[i] = *toAgent(&objects[i])
	}
	pr.Items = items
	return pr, nil
}

// ── Agent job APIs (token auth) ──
func (a *agentController) getAuthAgent(ctx context.Context, agentToken string) (*model.Agent, error) {
	obj, err := a.factory.Agent().GetBy(ctx, db.WithToken(strings.TrimSpace(agentToken)))
	if err != nil {
		klog.Errorf("failed to auth agent by token: %v", err)
		return nil, errors.NewError(fmt.Errorf("invalid agent token"), http.StatusUnauthorized)
	}
	if obj == nil {
		return nil, errors.NewError(fmt.Errorf("invalid agent token"), http.StatusUnauthorized)
	}
	klog.V(2).Infof("agent auth ok: id=%d name=%s", obj.Id, obj.Name)
	return obj, nil
}

func (a *agentController) Heartbeat(ctx context.Context, agentToken string, req *types.AgentHeartbeatRequest) error {
	obj, err := a.getAuthAgent(ctx, agentToken)
	if err != nil {
		return err
	}
	updates := map[string]interface{}{"status": model.AgentStatusOnline, "last_heartbeat": time.Now()}
	if req != nil {
		if req.Hostname != "" {
			updates["hostname"] = req.Hostname
		}
		if req.Version != "" {
			updates["version"] = req.Version
		}
	}
	return a.factory.Agent().InternalUpdate(ctx, obj.Id, updates)
}

func (a *agentController) Claim(ctx context.Context, agentToken string) (*types.Job, error) {
	// 只获取和自己相关的
	obj, err := a.getAuthAgent(ctx, agentToken)
	if err != nil {
		return nil, err
	}

	job, err := a.factory.Agent().Job().ClaimNext(ctx, obj.Id)
	if err != nil || job == nil {
		return nil, err
	}
	return &types.Job{
		Id:       job.Id,
		PlanId:   job.PlanId,
		AgentId:  job.AgentId,
		TaskName: job.TaskName,
		Kind:     job.Kind,
		Action:   job.Action,
		Image:    job.Image,
		Payload:  job.Payload,
		Status:   job.Status,
		Message:  job.Message,
	}, nil
}

func (a *agentController) AppendLogs(ctx context.Context, agentToken string, jobId int64, req *types.AgentJobLogsRequest) error {
	if _, err := a.getAuthAgent(ctx, agentToken); err != nil {
		return err
	}
	return a.factory.Agent().Job().AppendLogs(ctx, jobId, req.Chunk)
}

func (a *agentController) ReportResult(ctx context.Context, agentToken string, jobId int64, req *types.AgentJobResultRequest) error {
	if _, err := a.getAuthAgent(ctx, agentToken); err != nil {
		return err
	}
	status := model.JobSuccess
	if !req.Success {
		status = model.JobFailed
	}
	return a.factory.Agent().Job().InternalUpdate(ctx, jobId, map[string]interface{}{
		"status": status, "message": req.Message, "result": req.Result,
	})
}

func (a *agentController) BundlePath(ctx context.Context, agentToken string, jobId int64) (string, error) {
	obj, err := a.getAuthAgent(ctx, agentToken)
	if err != nil {
		return "", err
	}
	job, err := a.factory.Agent().Job().Get(ctx, jobId)
	if err != nil || job == nil || job.AgentId != obj.Id {
		return "", errors.NewError(fmt.Errorf("job not found"), http.StatusNotFound)
	}
	workDir := a.cc.Worker.WorkDir
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

// ── helpers ──

func buildAgentFilters(opt types.ListOptions) []db.Options {
	var opts []db.Options
	if opt.NameSelector != "" {
		opts = append(opts, db.WithNameLike(opt.NameSelector))
	}
	if opt.UserId != 0 {
		opts = append(opts, db.WithUser(opt.UserId))
	}
	if opt.AgentStatus != nil {
		opts = append(opts, db.WithStatus(*opt.AgentStatus))
	}
	return opts
}

func toAgent(o *model.Agent) *types.Agent {
	return &types.Agent{
		PixiuMeta:     types.PixiuMeta{Id: o.Id, ResourceVersion: o.ResourceVersion},
		TimeMeta:      types.TimeMeta{GmtCreate: o.GmtCreate, GmtModified: o.GmtModified},
		Name:          o.Name,
		Type:          o.AgentType,
		UserID:        o.UserID,
		Status:        o.Status,
		Hostname:      o.Hostname,
		Version:       o.Version,
		LastHeartbeat: o.LastHeartbeat,
		Description:   o.Description,
		Token:         o.Token,
	}
}
