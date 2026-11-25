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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

var (
	pipelineRunGVR = schema.GroupVersionResource{Group: "tekton.dev", Version: "v1", Resource: "pipelineruns"}
	taskRunGVR     = schema.GroupVersionResource{Group: "tekton.dev", Version: "v1", Resource: "taskruns"}
	podGVR         = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	secretGVR      = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
)

// RunSpec 引擎无关的运行参数，由 controller 从 pipeline config 组装。
type RunSpec struct {
	EngineName     string
	Namespace      string
	Config         types.PipelineConfig
	PipelineYaml   string
	SpecMode       string
	Template       string
	TimeoutSeconds int
	CredentialLabel string
}

// RunStatus 引擎无关的运行状态结果。
type RunStatus struct {
	Status     model.CICDRunStatus
	Message    string
	Digest     string
	CommitSha  string
	ImageTag   string
	Finished   bool
}

// PodRef 构建 Pod 引用，供日志采集使用。
type PodRef struct {
	Name      string
	Namespace string
	Step      string
}

// Engine 引擎适配层薄接口（设计第 4 节），Tekton 实现，未来加 Argo 不动业务代码。
type Engine interface {
	Name() string
	Status(ctx context.Context) (*types.CICDEngineStatus, error)
	CreateRun(ctx context.Context, spec RunSpec) (string, error)
	GetRun(ctx context.Context, namespace, engineName string) (*RunStatus, error)
	ListPods(ctx context.Context, namespace, engineName string) ([]PodRef, error)
	StreamLogs(ctx context.Context, namespace string, pod PodRef, follow bool) (io.ReadCloser, error)
	Cancel(ctx context.Context, namespace, engineName string) error
	// Credential 相关
	ListCredentials(ctx context.Context, namespace string) ([]types.CICDCredential, error)
	CreateCredential(ctx context.Context, namespace string, req *types.CreateCICDCredentialRequest) error
	DeleteCredential(ctx context.Context, namespace, name string) error
}

// TektonEngine 基于 dynamic client 的 Tekton 实现。
type TektonEngine struct {
	dynamic dynamic.Interface
}

func NewTektonEngine(cfg *rest.Config) (*TektonEngine, error) {
	dc, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &TektonEngine{dynamic: dc}, nil
}

func newTektonEngineWithClient(dc dynamic.Interface) *TektonEngine {
	return &TektonEngine{dynamic: dc}
}

func (e *TektonEngine) Name() string { return "tekton" }

// Status 通过查询 pipelineruns CRD 判断 Tekton 是否安装及版本。
func (e *TektonEngine) Status(ctx context.Context) (*types.CICDEngineStatus, error) {
	result := &types.CICDEngineStatus{Engine: "tekton"}
	list, err := e.dynamic.Resource(pipelineRunGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return result, nil
		}
		return nil, err
	}
	result.Installed = true
	// 从任一 pipelinerun 的 apiVersion 推断版本；无 run 时保持空版本。
	if len(list.Items) > 0 {
		if av, found, _ := unstructured.NestedString(list.Items[0].Object, "apiVersion"); found {
			parts := strings.Split(av, "/")
			if len(parts) == 2 {
				result.Version = parts[1]
			}
		}
	}
	return result, nil
}

// renderPipelineSpec 渲染 Tekton PipelineRun。template 模式内联 pipelineSpec，
// 遵循设计第 5 节：clone（git-clone）→ build-push（buildah）→ 输出 IMAGE_DIGEST result。
func renderPipelineSpec(spec RunSpec) (map[string]interface{}, error) {
	if spec.SpecMode == "raw" {
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(spec.PipelineYaml), &obj); err != nil {
			return nil, fmt.Errorf("parse raw pipeline yaml: %w", err)
		}
		return obj, nil
	}
	if spec.Template != "" && spec.Template != "source-build" {
		return nil, fmt.Errorf("unsupported template %q", spec.Template)
	}

	cfg := spec.Config
	gitRef := strings.TrimSpace(firstNonEmpty(cfg.Repo.Ref, "main"))
	dockerfile := firstNonEmpty(cfg.Build.Dockerfile, "Dockerfile")
	contextPath := firstNonEmpty(cfg.Build.Context, ".")
	image := strings.TrimSuffix(cfg.Image.Registry, "/")
	if cfg.Image.Repository != "" {
		image = strings.TrimSuffix(image, "/") + "/" + strings.TrimPrefix(cfg.Image.Repository, "/")
	}
	imageTag := firstNonEmpty(cfg.Image.Tag, "$(params.git-revision)")
	tlsVerify := cfg.Advanced.TLSVerify
	timeout := spec.TimeoutSeconds
	if timeout <= 0 {
		timeout = cfg.Advanced.TimeoutSeconds
	}
	if timeout <= 0 {
		timeout = 1200
	}

	buildArgs := make([]interface{}, 0, len(cfg.Build.Args))
	for _, a := range cfg.Build.Args {
		buildArgs = append(buildArgs, a)
	}

	// 凭据 Secret 只读挂载（设计 5.2）：git-secret / registry-secret 按名称引用。
	gitSecret := cfg.Repo.Credential
	regSecret := cfg.Image.Credential
	workspaceBindings := make([]interface{}, 0, 2)
	workspaces := make([]interface{}, 0, 2)
	volumeMounts := make([]interface{}, 0, 2)
	if gitSecret != "" {
		workspaces = append(workspaces, map[string]interface{}{"name": "git-auth"})
		workspaceBindings = append(workspaceBindings, map[string]interface{}{
			"name": "git-auth", "secret": map[string]interface{}{"secretName": gitSecret},
		})
	}
	if regSecret != "" {
		workspaces = append(workspaces, map[string]interface{}{"name": "registry-auth"})
		workspaceBindings = append(workspaceBindings, map[string]interface{}{
			"name": "registry-auth", "secret": map[string]interface{}{"secretName": regSecret},
		})
	}
	_ = volumeMounts

	cloneScript := strings.Join([]string{
		"set -eu",
		"git clone --depth 1 --branch $(params.git-revision) $(params.git-url) /workspace/source",
	}, "\n")

	buildScript := strings.Join([]string{
		"set +x",
		"cd /workspace/source",
		fmt.Sprintf("buildah bud --isolation=chroot --storage-driver=vfs --layers --tls-verify=%t \\", tlsVerify),
		fmt.Sprintf("  $(params.build-args) \\"),
		fmt.Sprintf("  -f %s -t $(params.image):$(params.image-tag) %s", dockerfile, contextPath),
		fmt.Sprintf("buildah push --tls-verify=%t --digestfile /tmp/digest $(params.image):$(params.image-tag)", tlsVerify),
		"cat /tmp/digest | cut -d: -f2 > /tmp/digest-sha",
	}, "\n")

	pipelineSpec := map[string]interface{}{
		"params": []interface{}{
			map[string]interface{}{"name": "git-url", "type": "string"},
			map[string]interface{}{"name": "git-revision", "type": "string"},
			map[string]interface{}{"name": "image", "type": "string"},
			map[string]interface{}{"name": "image-tag", "type": "string"},
			map[string]interface{}{"name": "build-args", "type": "array", "default": buildArgs},
		},
		"workspaces": workspaces,
		"tasks": []interface{}{
			map[string]interface{}{
				"name": "clone",
				"taskSpec": map[string]interface{}{
					"params": []interface{}{
						map[string]interface{}{"name": "git-url", "type": "string"},
						map[string]interface{}{"name": "git-revision", "type": "string"},
					},
					"workspaces": []interface{}{map[string]interface{}{"name": "source"}},
					"steps": []interface{}{
						map[string]interface{}{
							"name": "clone", "image": "alpine/git:v2.47.1",
							"script": cloneScript,
							"volumeMounts": []interface{}{
								map[string]interface{}{"name": "workspace-source", "mountPath": "/workspace/source"},
							},
						},
					},
				},
				"params": []interface{}{
					map[string]interface{}{"name": "git-url", "value": "$(params.git-url)"},
					map[string]interface{}{"name": "git-revision", "value": "$(params.git-revision)"},
				},
				"workspaces": []interface{}{map[string]interface{}{"name": "source", "workspace": "git-auth"}},
			},
			map[string]interface{}{
				"name": "build-push",
				"runAfter": []interface{}{"clone"},
				"taskSpec": map[string]interface{}{
					"params": []interface{}{
						map[string]interface{}{"name": "image", "type": "string"},
						map[string]interface{}{"name": "image-tag", "type": "string"},
						map[string]interface{}{"name": "build-args", "type": "array"},
					},
					"results": []interface{}{
						map[string]interface{}{"name": "IMAGE_DIGEST", "description": "构建产物 digest"},
					},
					"steps": []interface{}{
						map[string]interface{}{
							"name": "buildah", "image": "quay.io/buildah/stable:v1.39.0",
							"script": buildScript,
							"securityContext": map[string]interface{}{
								"runAsNonRoot": false,
								"allowPrivilegeEscalation": false,
								"capabilities": map[string]interface{}{"drop": []interface{}{"ALL"}},
								"seccompProfile": map[string]interface{}{"type": "RuntimeDefault"},
							},
						},
						map[string]interface{}{
							"name": "write-result",
							"image": "alpine:3.20",
							"script": "cat /tmp/digest-sha | tr -d '\\n' > $(results.IMAGE_DIGEST.path)",
						},
					},
				},
				"params": []interface{}{
					map[string]interface{}{"name": "image", "value": "$(params.image)"},
					map[string]interface{}{"name": "image-tag", "value": "$(params.image-tag)"},
					map[string]interface{}{"name": "build-args", "value": "$(params.build-args)"},
				},
			},
		},
	}

	pr := map[string]interface{}{
		"apiVersion": "tekton.dev/v1",
		"kind":       "PipelineRun",
		"metadata": map[string]interface{}{
			"name": spec.EngineName,
			"labels": map[string]interface{}{
				"pixiu.io/cicd": "true",
				"pixiu.io/engine-name": spec.EngineName,
			},
		},
		"spec": map[string]interface{}{
			"pipelineSpec": pipelineSpec,
			"params": []interface{}{
				map[string]interface{}{"name": "git-url", "value": cfg.Repo.URL},
				map[string]interface{}{"name": "git-revision", "value": gitRef},
				map[string]interface{}{"name": "image", "value": image},
				map[string]interface{}{"name": "image-tag", "value": imageTag},
				map[string]interface{}{"name": "build-args", "value": buildArgs},
			},
			"workspaces":     workspaceBindings,
			"timeouts":       map[string]interface{}{"pipeline": fmt.Sprintf("%ds", timeout)},
			"ttlSecondsAfterFinished": 86400,
		},
	}
	return pr, nil
}

// CreateRun 创建 PipelineRun，返回引擎对象名（确定性命名，支持对账）。
func (e *TektonEngine) CreateRun(ctx context.Context, spec RunSpec) (string, error) {
	obj, err := renderPipelineSpec(spec)
	if err != nil {
		return "", err
	}
	// 确定性命名：重复创建返回 AlreadyExists 时视为成功，便于「创建成功但回写失败」对账。
	_, err = e.dynamic.Resource(pipelineRunGVR).Namespace(spec.Namespace).Create(ctx, &unstructured.Unstructured{Object: obj}, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return "", err
	}
	return spec.EngineName, nil
}

// GetRun 查询 PipelineRun 状态并按设计 4 节映射。
func (e *TektonEngine) GetRun(ctx context.Context, namespace, engineName string) (*RunStatus, error) {
	obj, err := e.dynamic.Resource(pipelineRunGVR).Namespace(namespace).Get(ctx, engineName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return &RunStatus{Status: model.CICDRunFailed, Message: "PipelineRun 不存在或已被回收"}, nil
		}
		return nil, err
	}

	result := &RunStatus{Status: model.CICDRunRunning}

	// results → digest（IMAGE_DIGEST）
	if results, found, _ := unstructured.NestedSlice(obj.Object, "status", "results"); found {
		for _, r := range results {
			rm, ok := r.(map[string]interface{})
			if !ok {
				continue
			}
			if name, _ := rm["name"].(string); name == "IMAGE_DIGEST" {
				if value, _ := rm["value"].(string); strings.TrimSpace(value) != "" {
					result.Digest = "sha256:" + strings.TrimPrefix(strings.TrimSpace(value), "sha256:")
				}
			}
		}
	}

	conditions, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if !found || len(conditions) == 0 {
		return result, nil
	}
	cond, _ := conditions[0].(map[string]interface{})
	status, _ := cond["status"].(string)
	reason, _ := cond["reason"].(string)
	message, _ := cond["message"].(string)
	result.Message = message

	switch {
	case status == "True":
		result.Status = model.CICDRunSucceeded
		result.Finished = true
	case status == "False" && reason == "TimedOut":
		result.Status = model.CICDRunTimeout
		result.Finished = true
	case status == "False":
		result.Status = model.CICDRunFailed
		result.Finished = true
	default:
		result.Status = model.CICDRunRunning
	}
	return result, nil
}

// ListPods 按 label tekton.dev/pipelineRun=<engineName> 过滤 Pod（设计第 4 节）。
func (e *TektonEngine) ListPods(ctx context.Context, namespace, engineName string) ([]PodRef, error) {
	list, err := e.dynamic.Resource(podGVR).Namespace(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("tekton.dev/pipelineRun=%s", engineName),
	})
	if err != nil {
		return nil, err
	}
	refs := make([]PodRef, 0, len(list.Items))
	for _, item := range list.Items {
		step := item.GetLabels()["tekton.dev/pipelineTask"]
		if step == "" {
			step = "task"
		}
		refs = append(refs, PodRef{Name: item.GetName(), Namespace: item.GetNamespace(), Step: step})
	}
	return refs, nil
}

// StreamLogs 跟随指定 Pod 的容器日志（用户容器 tekton 固定名为 step-<step>）。
func (e *TektonEngine) StreamLogs(ctx context.Context, namespace string, pod PodRef, follow bool) (io.ReadCloser, error) {
	// dynamic client 不直接支持 pod logs，使用 RESTClient 需传入 rest.Config；
	// 这里保留接口，交由 controller 使用 clientset 的 PodLogs 实现包装（见 streamPodLogs）。
	return nil, fmt.Errorf("StreamLogs must be provided by a client with pod-log capability")
}

// Cancel 删除 PipelineRun（Tekton 原生取消语义）。
func (e *TektonEngine) Cancel(ctx context.Context, namespace, engineName string) error {
	err := e.dynamic.Resource(pipelineRunGVR).Namespace(namespace).Delete(ctx, engineName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

// ---- Credential (K8s Secret 元数据，永不返回值) ----

const credentialLabel = "pixiu.io/cicd-credential"

func (e *TektonEngine) ListCredentials(ctx context.Context, namespace string) ([]types.CICDCredential, error) {
	list, err := e.dynamic.Resource(secretGVR).Namespace(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: credentialLabel,
	})
	if err != nil {
		return nil, err
	}
	out := make([]types.CICDCredential, 0, len(list.Items))
	for _, item := range list.Items {
		out = append(out, types.CICDCredential{
			Name:      item.GetName(),
			Type:      item.GetLabels()[credentialLabel],
			Namespace: item.GetNamespace(),
			CreatedAt: item.GetCreationTimestamp().Format("2006-01-02T15:04:05Z"),
		})
	}
	return out, nil
}

func (e *TektonEngine) CreateCredential(ctx context.Context, namespace string, req *types.CreateCICDCredentialRequest) error {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]interface{}{
			"name": req.Name,
			"labels": map[string]interface{}{
				credentialLabel: req.Type,
			},
		},
		"type": "Opaque",
		"data": map[string]interface{}{
			"username": base64Encode(req.Username),
			"password": base64Encode(req.Password),
		},
	}}
	_, err := e.dynamic.Resource(secretGVR).Namespace(namespace).Create(ctx, obj, metav1.CreateOptions{})
	return err
}

func (e *TektonEngine) DeleteCredential(ctx context.Context, namespace, name string) error {
	err := e.dynamic.Resource(secretGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func base64Encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}
