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

package cicd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/caoyingjunz/pixiu/cmd/app/config"
	clustercontroller "github.com/caoyingjunz/pixiu/pkg/controller/cluster"
	controllerutil "github.com/caoyingjunz/pixiu/pkg/controller/util"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type Getter interface{ CICD() Interface }

// Interface CI/CD 控制面接口（设计第 8 节）。
type Interface interface {
	GetEngineStatus(ctx context.Context, clusterId int64) (*types.CICDEngineStatus, error)
	CreatePipeline(ctx context.Context, req *types.CreateCICDPipelineRequest) (*types.CICDPipeline, error)
	UpdatePipeline(ctx context.Context, id int64, req *types.UpdateCICDPipelineRequest) error
	DeletePipeline(ctx context.Context, id int64) error
	GetPipeline(ctx context.Context, id int64) (*types.CICDPipeline, error)
	ListPipelines(ctx context.Context, query *types.ListCICDPipelinesQuery) ([]types.CICDPipeline, int64, error)
	TriggerRun(ctx context.Context, id int64, req *types.RunPipelineRequest) (*types.CICDRun, error)
	ListRuns(ctx context.Context, query *types.ListCICDRunsQuery) ([]types.CICDRun, int64, error)
	GetRun(ctx context.Context, id int64) (*types.CICDRun, error)
	CancelRun(ctx context.Context, id int64) error
	RetryRun(ctx context.Context, id int64) (*types.CICDRun, error)
	WatchRunLogs(ctx context.Context, id int64, w http.ResponseWriter, r *http.Request) error

	ListCredentials(ctx context.Context, clusterId int64, namespace string) ([]types.CICDCredential, error)
	CreateCredential(ctx context.Context, clusterId int64, namespace string, req *types.CreateCICDCredentialRequest) error
	DeleteCredential(ctx context.Context, clusterId int64, namespace, name string) error

	// worker
	Run(ctx context.Context, workers int) error
}

type controller struct {
	cc       config.Config
	factory  db.ShareDaoFactory
	clusters clustercontroller.Interface
}

func New(cc config.Config, factory db.ShareDaoFactory, clusters clustercontroller.Interface) Interface {
	return &controller{cc: cc, factory: factory, clusters: clusters}
}

// ---- helpers ----

func (c *controller) pipeline(ctx context.Context, id int64) (*model.CICDPipeline, error) {
	p, err := c.factory.CICD().GetPipeline(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errors.ErrPipelineNotFound
	}
	return p, nil
}

func (c *controller) run(ctx context.Context, id int64) (*model.CICDRun, error) {
	r, err := c.factory.CICD().GetRun(ctx, id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, errors.ErrRunNotFound
	}
	return r, nil
}

// ensureClusterAccess 复用集群权限模型校验归属，不能只依赖集群 RBAC。
func (c *controller) ensureClusterAccess(ctx context.Context, clusterId int64) (*model.Cluster, error) {
	cluster, err := c.factory.Cluster().Get(ctx, clusterId)
	if err != nil {
		return nil, err
	}
	if cluster == nil {
		return nil, errors.ErrClusterNotFound
	}
	if err = controllerutil.CheckResourceAccess(ctx, c.factory, cluster.UserId, types.ResourceTypeCluster, cluster.Id); err != nil {
		return nil, err
	}
	return cluster, nil
}

// engine 构造 Tekton 引擎客户端（复用集群 rest.Config，含隧道）。
func (c *controller) engine(ctx context.Context, clusterId int64) (Engine, error) {
	cluster, err := c.factory.Cluster().Get(ctx, clusterId)
	if err != nil {
		return nil, err
	}
	if cluster == nil {
		return nil, errors.ErrClusterNotFound
	}
	clusterSet, err := c.clusters.GetClusterSetByName(ctx, cluster.Name)
	if err != nil {
		return nil, err
	}
	return NewTektonEngine(clusterSet.Config)
}

func (c *controller) GetEngineStatus(ctx context.Context, clusterId int64) (*types.CICDEngineStatus, error) {
	if _, err := c.ensureClusterAccess(ctx, clusterId); err != nil {
		return nil, err
	}
	engine, err := c.engine(ctx, clusterId)
	if err != nil {
		return nil, err
	}
	status, err := engine.Status(ctx)
	if err != nil {
		return nil, err
	}
	if !status.Installed {
		return nil, errors.ErrEngineNotInstalled
	}
	return status, nil
}

// ---- Pipeline CRUD ----

func (c *controller) CreatePipeline(ctx context.Context, req *types.CreateCICDPipelineRequest) (*types.CICDPipeline, error) {
	if _, err := c.ensureClusterAccess(ctx, req.ClusterId); err != nil {
		return nil, err
	}
	if req.Name == "" {
		return nil, errors.ErrReqParams
	}
	// 租户 + name 唯一（MVP 单租户：tenant_id=0，由归属人隔离）。
	existing, err := c.factory.CICD().GetPipelineByName(ctx, 0, req.Name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, errors.ErrPipelineExists
	}

	configBytes, err := json.Marshal(req.Config)
	if err != nil {
		return nil, err
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	specMode := req.SpecMode
	if specMode == "" {
		specMode = "template"
	}
	template := req.Template
	if template == "" {
		template = "source-build"
	}

	obj := &model.CICDPipeline{
		Name:           req.Name,
		Description:    req.Description,
		ClusterId:      req.ClusterId,
		Namespace:      req.Namespace,
		Engine:         "tekton",
		SpecMode:       specMode,
		Template:       template,
		Config:         string(configBytes),
		PipelineYaml:   req.PipelineYaml,
		TimeoutSeconds: req.TimeoutSeconds,
		Enabled:        enabled,
		CreatedBy:      req.UserId,
		CreatedByName:  req.UserName,
	}
	created, err := c.factory.CICD().CreatePipeline(ctx, obj)
	if err != nil {
		return nil, err
	}
	return toPipeline(created), nil
}

func (c *controller) UpdatePipeline(ctx context.Context, id int64, req *types.UpdateCICDPipelineRequest) error {
	p, err := c.pipeline(ctx, id)
	if err != nil {
		return err
	}
	if _, err = c.ensureClusterAccess(ctx, p.ClusterId); err != nil {
		return err
	}
	if err = controllerutil.CheckResourceOwner(ctx, p.CreatedBy); err != nil {
		return err
	}
	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.ClusterId != nil {
		if _, err = c.ensureClusterAccess(ctx, *req.ClusterId); err != nil {
			return err
		}
		updates["cluster_id"] = *req.ClusterId
	}
	if req.Namespace != nil {
		updates["namespace"] = *req.Namespace
	}
	if req.Config != nil {
		b, err := json.Marshal(req.Config)
		if err != nil {
			return err
		}
		updates["config"] = string(b)
	}
	if req.PipelineYaml != nil {
		updates["pipeline_yaml"] = *req.PipelineYaml
	}
	if req.TimeoutSeconds != nil {
		updates["timeout_seconds"] = *req.TimeoutSeconds
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if len(updates) == 0 {
		return nil
	}
	ok, err := c.factory.CICD().UpdatePipeline(ctx, id, req.ResourceVersion, updates)
	if err != nil {
		return err
	}
	if !ok {
		return errors.ErrConflict
	}
	return nil
}

func (c *controller) DeletePipeline(ctx context.Context, id int64) error {
	p, err := c.pipeline(ctx, id)
	if err != nil {
		return err
	}
	if err = controllerutil.CheckResourceOwner(ctx, p.CreatedBy); err != nil {
		return err
	}
	// 存在非终态 run 时禁止删除，避免悬空。
	running, err := c.factory.CICD().CountRunningByPipeline(ctx, id)
	if err != nil {
		return err
	}
	if running > 0 {
		return errors.ErrNotAcceptable
	}
	return c.factory.CICD().DeletePipeline(ctx, id)
}

func (c *controller) GetPipeline(ctx context.Context, id int64) (*types.CICDPipeline, error) {
	p, err := c.pipeline(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err = c.ensureClusterAccess(ctx, p.ClusterId); err != nil {
		return nil, err
	}
	return toPipeline(p), nil
}

func (c *controller) ListPipelines(ctx context.Context, query *types.ListCICDPipelinesQuery) ([]types.CICDPipeline, int64, error) {
	opts := []db.Options{}
	if query.ClusterId != 0 {
		opts = append(opts, db.WithClusterId(query.ClusterId))
	}
	if query.Namespace != "" {
		opts = append(opts, db.WithCICDPipelineNamespace(query.Namespace))
	}
	if query.Name != "" {
		opts = append(opts, db.WithNameLike(query.Name))
	}
	// 非 root 用户仅可见自己创建的流水线。
	if !isRoot(ctx) {
		uid, err := currentUserID(ctx)
		if err != nil {
			return nil, 0, err
		}
		opts = append(opts, db.WithCICDCreatedBy(uid))
	}
	total, err := c.factory.CICD().CountPipelines(ctx, opts...)
	if err != nil {
		return nil, 0, err
	}
	if query.PageSize > 0 {
		if query.Page <= 0 {
			query.Page = 1
		}
		opts = append(opts, db.WithOffset((query.Page-1)*query.PageSize), db.WithLimit(query.PageSize))
	}
	items, err := c.factory.CICD().ListPipelines(ctx, opts...)
	if err != nil {
		return nil, 0, err
	}
	result := make([]types.CICDPipeline, 0, len(items))
	for i := range items {
		result = append(result, *toPipeline(&items[i]))
	}
	return result, total, nil
}

// ---- Run 操作 ----

func (c *controller) TriggerRun(ctx context.Context, id int64, req *types.RunPipelineRequest) (*types.CICDRun, error) {
	p, err := c.pipeline(ctx, id)
	if err != nil {
		return nil, err
	}
	if !p.Enabled {
		return nil, fmt.Errorf("流水线已禁用")
	}
	if _, err = c.ensureClusterAccess(ctx, p.ClusterId); err != nil {
		return nil, err
	}
	// 单流水线并发限制。
	maxRunning := c.maxRunningPerPipeline()
	running, err := c.factory.CICD().CountRunningByPipeline(ctx, id)
	if err != nil {
		return nil, err
	}
	if running >= maxRunning {
		return nil, errors.ErrRunConflict
	}
	// pending 上限。
	maxPending := c.maxPendingRuns()
	if maxPending > 0 {
		pending, err := c.factory.CICD().CountPending(ctx)
		if err != nil {
			return nil, err
		}
		if pending >= maxPending {
			return nil, errors.ErrRunConflict
		}
	}

	ref := strings.TrimSpace(req.Ref)
	if ref == "" {
		var cfg types.PipelineConfig
		if p.Config != "" {
			_ = json.Unmarshal([]byte(p.Config), &cfg)
		}
		ref = cfg.Repo.Ref
	}
	if ref == "" {
		ref = "main"
	}

	paramsSnapshot := map[string]interface{}{
		"ref": req.Ref,
		"tag": req.Tag,
	}
	paramsBytes, _ := json.Marshal(paramsSnapshot)
	engineName := fmt.Sprintf("px-%d-", p.Id) // run id 在 create 后补全

	obj := &model.CICDRun{
		PipelineId:    p.Id,
		TenantId:      p.TenantId,
		ClusterId:     p.ClusterId,
		Namespace:     p.Namespace,
		TriggerType:   model.CICDTriggerManual,
		Status:        model.CICDRunPending,
		Engine:        "tekton",
		GitRef:        ref,
		Params:        string(paramsBytes),
		CreatedBy:     req.UserId,
		CreatedByName: req.UserName,
	}
	// 先建 run 拿到 id，再回填确定性 engine_name。
	created, err := c.factory.CICD().CreateRun(ctx, obj)
	if err != nil {
		return nil, err
	}
	engineName = fmt.Sprintf("px-%d-%d", p.Id, created.Id)
	if err = c.factory.CICD().UpdateRun(ctx, created.Id, map[string]interface{}{"engine_name": engineName}); err != nil {
		return nil, err
	}
	created.EngineName = engineName
	return toRun(created), nil
}

func (c *controller) ListRuns(ctx context.Context, query *types.ListCICDRunsQuery) ([]types.CICDRun, int64, error) {
	opts := []db.Options{}
	if query.PipelineId != 0 {
		opts = append(opts, db.WithCICDPipelineId(query.PipelineId))
	}
	if query.Status != "" {
		opts = append(opts, db.WithCICDRunStatus(query.Status))
	}
	if !isRoot(ctx) {
		uid, err := currentUserID(ctx)
		if err != nil {
			return nil, 0, err
		}
		opts = append(opts, db.WithCICDCreatedBy(uid))
	}
	total, err := c.factory.CICD().CountRuns(ctx, opts...)
	if err != nil {
		return nil, 0, err
	}
	if query.PageSize > 0 {
		if query.Page <= 0 {
			query.Page = 1
		}
		opts = append(opts, db.WithOffset((query.Page-1)*query.PageSize), db.WithLimit(query.PageSize))
	}
	items, err := c.factory.CICD().ListRuns(ctx, opts...)
	if err != nil {
		return nil, 0, err
	}
	result := make([]types.CICDRun, 0, len(items))
	for i := range items {
		result = append(result, *toRun(&items[i]))
	}
	return result, total, nil
}

func (c *controller) GetRun(ctx context.Context, id int64) (*types.CICDRun, error) {
	r, err := c.run(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err = c.ensureClusterAccess(ctx, r.ClusterId); err != nil {
		return nil, err
	}
	return toRun(r), nil
}

func (c *controller) CancelRun(ctx context.Context, id int64) error {
	r, err := c.run(ctx, id)
	if err != nil {
		return err
	}
	if err = controllerutil.CheckResourceOwner(ctx, r.CreatedBy); err != nil {
		return err
	}
	if r.Status.IsTerminal() {
		return errors.ErrNotAcceptable
	}
	// 未执行的 pending 直接取消；running 调引擎删除 PipelineRun。
	if r.Status == model.CICDRunRunning {
		engine, err := c.engine(ctx, r.ClusterId)
		if err != nil {
			return err
		}
		if err = engine.Cancel(ctx, r.Namespace, r.EngineName); err != nil {
			return err
		}
	}
	now := c.now()
	return c.factory.CICD().UpdateRun(ctx, id, map[string]interface{}{
		"status":      model.CICDRunCancelled,
		"message":     "用户取消",
		"finished_at": now,
	})
}

func (c *controller) RetryRun(ctx context.Context, id int64) (*types.CICDRun, error) {
	old, err := c.run(ctx, id)
	if err != nil {
		return nil, err
	}
	if !old.Status.IsTerminal() {
		return nil, errors.ErrNotAcceptable
	}
	if err = controllerutil.CheckResourceOwner(ctx, old.CreatedBy); err != nil {
		return nil, err
	}
	obj := &model.CICDRun{
		PipelineId:    old.PipelineId,
		TenantId:      old.TenantId,
		ClusterId:     old.ClusterId,
		Namespace:     old.Namespace,
		TriggerType:   old.TriggerType,
		Status:        model.CICDRunPending,
		Engine:        old.Engine,
		GitRef:        old.GitRef,
		Params:        old.Params,
		CreatedBy:     old.CreatedBy,
		CreatedByName: old.CreatedByName,
	}
	created, err := c.factory.CICD().CreateRun(ctx, obj)
	if err != nil {
		return nil, err
	}
	engineName := fmt.Sprintf("px-%d-%d", old.PipelineId, created.Id)
	if err = c.factory.CICD().UpdateRun(ctx, created.Id, map[string]interface{}{"engine_name": engineName}); err != nil {
		return nil, err
	}
	created.EngineName = engineName
	return toRun(created), nil
}

// ---- Credentials ----

func (c *controller) ListCredentials(ctx context.Context, clusterId int64, namespace string) ([]types.CICDCredential, error) {
	if _, err := c.ensureClusterAccess(ctx, clusterId); err != nil {
		return nil, err
	}
	engine, err := c.engine(ctx, clusterId)
	if err != nil {
		return nil, err
	}
	return engine.ListCredentials(ctx, namespace)
}

func (c *controller) CreateCredential(ctx context.Context, clusterId int64, namespace string, req *types.CreateCICDCredentialRequest) error {
	if _, err := c.ensureClusterAccess(ctx, clusterId); err != nil {
		return err
	}
	engine, err := c.engine(ctx, clusterId)
	if err != nil {
		return err
	}
	return engine.CreateCredential(ctx, namespace, req)
}

func (c *controller) DeleteCredential(ctx context.Context, clusterId int64, namespace, name string) error {
	if _, err := c.ensureClusterAccess(ctx, clusterId); err != nil {
		return err
	}
	engine, err := c.engine(ctx, clusterId)
	if err != nil {
		return err
	}
	return engine.DeleteCredential(ctx, namespace, name)
}

// ---- converters ----

func toPipeline(p *model.CICDPipeline) *types.CICDPipeline {
	var cfg types.PipelineConfig
	if p.Config != "" {
		_ = json.Unmarshal([]byte(p.Config), &cfg)
	}
	return &types.CICDPipeline{
		PixiuMeta:      types.PixiuMeta{Id: p.Id, ResourceVersion: p.ResourceVersion},
		TimeMeta:       types.TimeMeta{GmtCreate: p.GmtCreate, GmtModified: p.GmtModified},
		Name:           p.Name,
		Description:    p.Description,
		ClusterId:      p.ClusterId,
		Namespace:      p.Namespace,
		Engine:         p.Engine,
		SpecMode:       p.SpecMode,
		Template:       p.Template,
		Config:         cfg,
		PipelineYaml:   p.PipelineYaml,
		TimeoutSeconds: p.TimeoutSeconds,
		Enabled:        p.Enabled,
		CreatedBy:      p.CreatedBy,
		CreatedByName:  p.CreatedByName,
	}
}

func toRun(r *model.CICDRun) *types.CICDRun {
	return &types.CICDRun{
		PixiuMeta:     types.PixiuMeta{Id: r.Id, ResourceVersion: r.ResourceVersion},
		TimeMeta:      types.TimeMeta{GmtCreate: r.GmtCreate, GmtModified: r.GmtModified},
		PipelineId:    r.PipelineId,
		TenantId:      r.TenantId,
		ClusterId:     r.ClusterId,
		Namespace:     r.Namespace,
		TriggerType:   r.TriggerType,
		Status:        r.Status,
		Engine:        r.Engine,
		EngineName:    r.EngineName,
		GitRef:        r.GitRef,
		CommitSha:     r.CommitSha,
		ImageRepo:     r.ImageRepo,
		ImageTag:      r.ImageTag,
		ImageDigest:   r.ImageDigest,
		Params:        r.Params,
		Message:       r.Message,
		DurationMs:    r.DurationMs,
		StartedAt:     r.StartedAt,
		FinishedAt:    r.FinishedAt,
		CreatedBy:     r.CreatedBy,
		CreatedByName: r.CreatedByName,
	}
}
