package types

import (
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

// ---- CICD Pipeline ----

type PipelineConfigRepo struct {
	URL        string `json:"url"`
	Ref        string `json:"ref"`
	Credential string `json:"credential"`
}

type PipelineConfigBuild struct {
	Dockerfile string   `json:"dockerfile"`
	Context    string   `json:"context"`
	Args       []string `json:"args"`
}

type PipelineConfigImage struct {
	Registry   string `json:"registry"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Credential string `json:"credential"`
}

type PipelineConfigAdvanced struct {
	TimeoutSeconds int  `json:"timeout_seconds"`
	TLSVerify      bool `json:"tls_verify"`
}

type PipelineConfig struct {
	Repo     PipelineConfigRepo     `json:"repo"`
	Build    PipelineConfigBuild    `json:"build"`
	Image    PipelineConfigImage    `json:"image"`
	Advanced PipelineConfigAdvanced `json:"advanced"`
}

type CICDPipeline struct {
	PixiuMeta   `json:",inline"`
	TimeMeta    `json:",inline"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ClusterId   int64  `json:"cluster_id"`
	Namespace   string `json:"namespace"`
	Engine      string `json:"engine"`
	SpecMode    string `json:"spec_mode"`
	Template    string `json:"template"`
	Config      PipelineConfig `json:"config"`
	PipelineYaml string `json:"pipeline_yaml,omitempty"`
	TimeoutSeconds int `json:"timeout_seconds"`
	Enabled     bool   `json:"enabled"`
	CreatedBy   int64  `json:"created_by"`
	CreatedByName string `json:"created_by_name"`
}

type CreateCICDPipelineRequest struct {
	Name        string         `json:"name" binding:"required,max=128"`
	Description string         `json:"description"`
	ClusterId   int64          `json:"cluster_id" binding:"required"`
	Namespace   string         `json:"namespace" binding:"required,max=128"`
	Template    string         `json:"template"`
	SpecMode    string         `json:"spec_mode"`
	Config      PipelineConfig `json:"config" binding:"required"`
	PipelineYaml string        `json:"pipeline_yaml"`
	TimeoutSeconds int         `json:"timeout_seconds"`
	Enabled     *bool          `json:"enabled"`
	UserId      int64          `json:"-"`
	UserName    string         `json:"-"`
}

func (r *CreateCICDPipelineRequest) SetUserID(id int64) { r.UserId = id }

type UpdateCICDPipelineRequest struct {
	ResourceVersion int64          `json:"resource_version" binding:"required"`
	Name        *string          `json:"name"`
	Description *string          `json:"description"`
	ClusterId   *int64           `json:"cluster_id"`
	Namespace   *string          `json:"namespace"`
	Config      *PipelineConfig  `json:"config"`
	PipelineYaml *string         `json:"pipeline_yaml"`
	TimeoutSeconds *int          `json:"timeout_seconds"`
	Enabled     *bool            `json:"enabled"`
}

// ---- CICD Run ----

type CICDRun struct {
	PixiuMeta     `json:",inline"`
	TimeMeta      `json:",inline"`
	PipelineId    int64              `json:"pipeline_id"`
	TenantId      int64              `json:"tenant_id"`
	ClusterId     int64              `json:"cluster_id"`
	Namespace     string             `json:"namespace"`
	TriggerType   model.CICDTriggerType `json:"trigger_type"`
	Status        model.CICDRunStatus   `json:"status"`
	Engine        string             `json:"engine"`
	EngineName    string             `json:"engine_name"`
	GitRef        string             `json:"git_ref"`
	CommitSha     string             `json:"commit_sha"`
	ImageRepo     string             `json:"image_repo"`
	ImageTag      string             `json:"image_tag"`
	ImageDigest   string             `json:"image_digest"`
	Params        string             `json:"params,omitempty"`
	Message       string             `json:"message"`
	DurationMs    int64              `json:"duration_ms"`
	StartedAt     *time.Time         `json:"started_at"`
	FinishedAt    *time.Time         `json:"finished_at"`
	CreatedBy     int64              `json:"created_by"`
	CreatedByName string             `json:"created_by_name"`
}

type RunPipelineRequest struct {
	Ref    string `json:"ref"`
	Tag    string `json:"tag"`
	UserId int64  `json:"-"`
	UserName string `json:"-"`
}

func (r *RunPipelineRequest) SetUserID(id int64) { r.UserId = id }

type ListCICDRunsQuery struct {
	PipelineId int64               `form:"pipeline_id" json:"pipeline_id"`
	Status     model.CICDRunStatus `form:"status" json:"status"`
	Page       int                 `form:"page" json:"page"`
	PageSize   int                 `form:"page_size" json:"page_size"`
}

type ListCICDPipelinesQuery struct {
	ClusterId int64  `form:"cluster_id" json:"cluster_id"`
	Namespace string `form:"namespace" json:"namespace"`
	Name      string `form:"name" json:"name"`
	Page      int    `form:"page" json:"page"`
	PageSize  int    `form:"page_size" json:"page_size"`
}

// ---- CICD Engine status ----

type CICDEngineStatus struct {
	Installed bool   `json:"installed"`
	Version   string `json:"version"`
	Engine    string `json:"engine"`
}

// ---- CICD Credential (K8s Secret metadata, no values) ----

type CICDCredential struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	CreatedAt  string `json:"created_at"`
	Namespace  string `json:"namespace"`
}

type CreateCICDCredentialRequest struct {
	Name     string `json:"name" binding:"required,max=128"`
	Type     string `json:"type" binding:"required,oneof=git registry"`
	Username string `json:"username"`
	Password string `json:"password" binding:"required"`
}
