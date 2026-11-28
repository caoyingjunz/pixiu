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

// Package networkpolicy 提供 networking.k8s.io/v1 NetworkPolicy 的增删改查能力（issue #831）。
//
// 设计要点：
//   - 只操作标准 NetworkPolicy，不引入自定义 CRD；
//   - 对象的名称/命名空间取自 path 参数，不接受 body 覆盖，避免歧义；
//   - 更新时带上 resourceVersion 做冲突检测，避免静默覆盖他人修改；
//   - apiserver 的典型错误（NotFound/AlreadyExists/Conflict/Forbidden/Invalid）映射为对应 HTTP 语义。
package networkpolicy

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	clustercontroller "github.com/caoyingjunz/pixiu/pkg/controller/cluster"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"

	networkingv1 "k8s.io/api/networking/v1"
)

const objectKind = "networkpolicy"

type Getter interface{ NetworkPolicy() Interface }

type Interface interface {
	// List 列出指定命名空间下的 NetworkPolicy
	List(ctx context.Context, user *model.User, clusterId int64, namespace string) (*networkingv1.NetworkPolicyList, error)
	// Get 获取单个 NetworkPolicy
	Get(ctx context.Context, user *model.User, clusterId int64, namespace, name string) (*networkingv1.NetworkPolicy, error)
	// Create 创建 NetworkPolicy
	Create(ctx context.Context, user *model.User, clusterId int64, namespace string, req *types.NetworkPolicyCreateRequest) (*networkingv1.NetworkPolicy, error)
	// Update 更新 NetworkPolicy 的元数据与 spec
	Update(ctx context.Context, user *model.User, clusterId int64, namespace, name string, req *types.NetworkPolicyUpdateRequest) (*networkingv1.NetworkPolicy, error)
	// Delete 删除 NetworkPolicy
	Delete(ctx context.Context, user *model.User, clusterId int64, namespace, name string) error
}

type controller struct {
	clusters clustercontroller.Interface
}

func New(clusters clustercontroller.Interface) Interface {
	return &controller{clusters: clusters}
}

func (c *controller) List(ctx context.Context, user *model.User, clusterId int64, namespace string) (*networkingv1.NetworkPolicyList, error) {
	client, err := c.client(ctx, user, clusterId)
	if err != nil {
		return nil, err
	}
	if err = validateNamespace(namespace); err != nil {
		return nil, err
	}

	list, err := client.NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, wrapK8sError(err, namespace)
	}
	return list, nil
}

func (c *controller) Get(ctx context.Context, user *model.User, clusterId int64, namespace, name string) (*networkingv1.NetworkPolicy, error) {
	client, err := c.client(ctx, user, clusterId)
	if err != nil {
		return nil, err
	}
	if err = validateNamespace(namespace); err != nil {
		return nil, err
	}
	if err = validateObjectName(name); err != nil {
		return nil, err
	}

	policy, err := client.NetworkingV1().NetworkPolicies(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, wrapK8sError(err, name)
	}
	return policy, nil
}

func (c *controller) Create(ctx context.Context, user *model.User, clusterId int64, namespace string, req *types.NetworkPolicyCreateRequest) (*networkingv1.NetworkPolicy, error) {
	client, err := c.client(ctx, user, clusterId)
	if err != nil {
		return nil, err
	}
	return createNetworkPolicy(ctx, client, namespace, req)
}

func (c *controller) Update(ctx context.Context, user *model.User, clusterId int64, namespace, name string, req *types.NetworkPolicyUpdateRequest) (*networkingv1.NetworkPolicy, error) {
	client, err := c.client(ctx, user, clusterId)
	if err != nil {
		return nil, err
	}
	return updateNetworkPolicy(ctx, client, namespace, name, req)
}

func (c *controller) Delete(ctx context.Context, user *model.User, clusterId int64, namespace, name string) error {
	client, err := c.client(ctx, user, clusterId)
	if err != nil {
		return err
	}
	return deleteNetworkPolicy(ctx, client, namespace, name)
}

// client 按集群 id 鉴权并返回该集群的客户端。
// 使用 AuthorizeClusterAccess：非 owner/root 的用户会按其授权子集群的 scoped kubeconfig 访问，
// 与集群资源页的权限模型保持一致。
func (c *controller) client(ctx context.Context, user *model.User, clusterId int64) (kubernetes.Interface, error) {
	credCluster, err := c.clusters.AuthorizeClusterAccess(ctx, user, clusterId)
	if err != nil {
		return nil, err
	}
	clusterSet, err := c.clusters.GetClusterSetByName(ctx, credCluster.Name)
	if err != nil {
		return nil, err
	}
	return clusterSet.Client, nil
}

// createNetworkPolicy / updateNetworkPolicy / deleteNetworkPolicy 与客户端解耦，便于用 fake clientset 做单测。

func createNetworkPolicy(ctx context.Context, client kubernetes.Interface, namespace string, req *types.NetworkPolicyCreateRequest) (*networkingv1.NetworkPolicy, error) {
	policy, err := buildNetworkPolicy(namespace, req)
	if err != nil {
		return nil, err
	}

	created, err := client.NetworkingV1().NetworkPolicies(namespace).Create(ctx, policy, metav1.CreateOptions{})
	if err != nil {
		return nil, wrapK8sError(err, req.Name)
	}
	return created, nil
}

func updateNetworkPolicy(ctx context.Context, client kubernetes.Interface, namespace, name string, req *types.NetworkPolicyUpdateRequest) (*networkingv1.NetworkPolicy, error) {
	if err := validateNamespace(namespace); err != nil {
		return nil, err
	}
	if err := validateObjectName(name); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, fmt.Errorf("请求体不能为空")
	}

	current, err := client.NetworkingV1().NetworkPolicies(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, wrapK8sError(err, name)
	}

	updated := current.DeepCopy()
	updated.Spec = req.Spec
	updated.Labels = req.Labels
	updated.Annotations = req.Annotations
	// 显式传入旧版本时保留，交由 apiserver 做冲突检测（返回 409）
	if req.ResourceVersion != "" {
		updated.ResourceVersion = req.ResourceVersion
	}

	result, err := client.NetworkingV1().NetworkPolicies(namespace).Update(ctx, updated, metav1.UpdateOptions{})
	if err != nil {
		return nil, wrapK8sError(err, name)
	}
	return result, nil
}

func deleteNetworkPolicy(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	if err := validateNamespace(namespace); err != nil {
		return err
	}
	if err := validateObjectName(name); err != nil {
		return err
	}

	propagation := metav1.DeletePropagationBackground
	if err := client.NetworkingV1().NetworkPolicies(namespace).Delete(ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &propagation,
	}); err != nil {
		return wrapK8sError(err, name)
	}
	return nil
}

// buildNetworkPolicy 由请求构造对象：名称与命名空间以 path 为准，不信任 body。
func buildNetworkPolicy(namespace string, req *types.NetworkPolicyCreateRequest) (*networkingv1.NetworkPolicy, error) {
	if req == nil {
		return nil, fmt.Errorf("请求体不能为空")
	}
	if err := validateNamespace(namespace); err != nil {
		return nil, err
	}
	if err := validateObjectName(req.Name); err != nil {
		return nil, err
	}

	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:        req.Name,
			Namespace:   namespace,
			Labels:      req.Labels,
			Annotations: req.Annotations,
		},
		Spec: req.Spec,
	}, nil
}

func validateNamespace(namespace string) error {
	if namespace == "" {
		return fmt.Errorf("namespace 不能为空")
	}
	if errs := validation.IsDNS1123Label(namespace); len(errs) > 0 {
		return fmt.Errorf("namespace %q 不合法: %s", namespace, strings.Join(errs, "; "))
	}
	return nil
}

func validateObjectName(name string) error {
	if name == "" {
		return fmt.Errorf("name 不能为空")
	}
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		return fmt.Errorf("name %q 不合法: %s", name, strings.Join(errs, "; "))
	}
	return nil
}

// wrapK8sError 把 apiserver 的典型错误映射为对应的 HTTP 语义，避免一律返回 400。
func wrapK8sError(err error, name string) error {
	switch {
	case apierrors.IsNotFound(err):
		return errors.NewError(fmt.Errorf("%s %q 不存在", objectKind, name), http.StatusNotFound)
	case apierrors.IsAlreadyExists(err):
		return errors.NewError(fmt.Errorf("%s %q 已存在", objectKind, name), http.StatusConflict)
	case apierrors.IsConflict(err):
		return errors.NewError(fmt.Errorf("%s %q 已被修改，请刷新后重试", objectKind, name), http.StatusConflict)
	case apierrors.IsForbidden(err):
		return errors.NewError(err, http.StatusForbidden)
	case apierrors.IsInvalid(err):
		return errors.NewError(err, http.StatusBadRequest)
	}
	return err
}
