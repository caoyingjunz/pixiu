package cluster

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/caoyingjunz/pixiu/pkg/client"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/caoyingjunz/pixiu/pkg/util/errors"
)

const (
	defaultExpiration        = 365 * 24 * time.Hour
	defaultExpirationSeconds = int64(defaultExpiration / time.Second)
	defaultNamespace         = "pixiu-system"
)

// CreatePermission 创建 scoped kubeconfig 并持久化
// TODO 后续优化
func (c *cluster) CreatePermission(ctx context.Context, req *types.CreatePermissionRequest) error {
	// 用户id 必须存在
	if req.UserId == 0 {
		return errors.ErrReqParams
	}

	if req.ExpirationSeconds <= 0 {
		req.ExpirationSeconds = defaultExpirationSeconds
	}

	// 创建授权 k8s 集群
	old, err := c.factory.Cluster().Get(ctx, req.ClusterId)
	if err != nil {
		klog.Errorf("获取主集(%d)群属性失败 %v", req.ClusterId, err)
		return err
	}

	saNs := req.SANamespace
	if saNs == "" {
		saNs = defaultNamespace
	}
	saName := req.SAName
	if saName == "" {
		saName = fmt.Sprintf("pixiu-sa-%d", req.UserId)
	}
	clusterSet, err := c.GetClusterSetByName(ctx, old.Name)
	if err != nil {
		return fmt.Errorf("获取集群 %s 配置失败: %w", old.Name, err)
	}

	rulesJSON, err := encodeRules(req.Rules)
	if err != nil {
		return err
	}
	nsJSON, err := encodeStringSlice(req.TargetNamespaces)
	if err != nil {
		return err
	}

	p, err := c.factory.Permission().Create(ctx, &model.Permission{
		UserId:            req.UserId,
		Name:              req.Name,
		ExpirationSeconds: req.ExpirationSeconds,
		PType:             model.PermissionPType(req.PType),
		Rules:             rulesJSON,
		SAName:            saName,
		SANamespace:       saNs,
		TargetNamespaces:  nsJSON,
		Description:       req.Description,
	})
	if err != nil {
		// TODO: 移除 k8s 已下发资源
		if errors.IsUniqueConstraintError(err) {
			return fmt.Errorf("同一用户在该集群下已存在同名授权")
		}
		klog.Errorf("failed to create permission: %v", err)
		return errors.ErrInternal
	}

	kubeConfig, err := c.addKubernetesRule(ctx, req, clusterSet, saNs, saName)
	if err != nil {
		return err
	}

	newObj, err := c.Create(ctx, &types.CreateClusterRequest{
		AliasName:      old.AliasName,
		UserId:         req.UserId,
		Type:           old.ClusterType,
		KubeConfig:     kubeConfig,
		Protected:      old.Protected,
		Description:    old.Description,
		PermissionId:   p.Id,
		OwnerReference: old.Id,
	})
	if err != nil {
		// todo 清理过程脏数据
		klog.Errorf("创建授权集群失败")
		return err
	}

	// 更新权限的集群id
	// TODO: 内部更新，可忽略 ResourceVersion
	if err = c.factory.Permission().Update(ctx, p.Id, p.ResourceVersion, map[string]interface{}{
		"cluster_id": newObj.Id,
	}); err != nil {
		klog.Errorf("更新权限集群失败 %v", err)
		return err
	}

	return nil
}

func (c *cluster) GetPermission(ctx context.Context, permissionId int64) (*types.Permission, error) {
	object, err := c.factory.Permission().Get(ctx, permissionId)
	if err != nil {
		klog.Errorf("failed to get permission(%d): %v", permissionId, err)
		return nil, errors.ErrInternal
	}
	if object == nil {
		return nil, errors.ErrPermissionNotFound
	}
	return c.permissionModel2Type(object), nil
}

func (c *cluster) ListPermissions(ctx context.Context, listOption types.ListOptions) (interface{}, error) {
	listOption.SetDefaultPageOption()

	pageResult := types.PageResult{
		PageRequest: types.PageRequest{
			Page:  listOption.Page,
			Limit: listOption.Limit,
		},
	}

	opts := []db.Options{
		db.WithNameLike(listOption.NameSelector),
	}

	var err error
	pageResult.Total, err = c.factory.Permission().Count(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to count permissions: %v", err)
		pageResult.Message = err.Error()
	}

	offset := (listOption.Page - 1) * listOption.Limit
	opts = append(opts, []db.Options{
		db.WithModifyOrderByDesc(),
		db.WithOffset(offset),
		db.WithLimit(listOption.Limit),
	}...)

	objects, err := c.factory.Permission().List(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to list permissions: %v", err)
		pageResult.Message = err.Error()
		return nil, errors.ErrInternal
	}

	items := make([]types.Permission, 0)
	for _, o := range objects {
		items = append(items, *c.permissionModel2Type(&o))
	}
	pageResult.Items = items

	return pageResult, nil
}

func (c *cluster) UpdatePermission(ctx context.Context, permissionId int64, req *types.UpdatePermissionRequest) error {
	//object, err := c.factory.Permission().Get(ctx, permissionId)
	//if err != nil {
	//	klog.Errorf("failed to get permission(%d): %v", permissionId, err)
	//	return errors.ErrInternal
	//}
	//if object == nil {
	//	return errors.ErrPermissionNotFound
	//}
	//
	//updates := make(map[string]interface{})
	//if req.Name != nil {
	//	updates["name"] = *req.Name
	//}
	//if req.Description != nil {
	//	updates["description"] = *req.Description
	//}
	//if req.ExpirationSeconds != nil {
	//	updates["expiration_seconds"] = *req.ExpirationSeconds
	//}
	//if req.PType != nil {
	//	updates["p_type"] = model.PermissionPType(*req.PType)
	//}
	//if req.Rules != nil {
	//	rulesJSON, err := encodeRules(req.Rules)
	//	if err != nil {
	//		return err
	//	}
	//	updates["rules"] = rulesJSON
	//}
	//if req.TargetNamespaces != nil {
	//	nsJSON, err := encodeStringSlice(req.TargetNamespaces)
	//	if err != nil {
	//		return err
	//	}
	//	updates["target_namespaces"] = nsJSON
	//}
	//if len(updates) == 0 {
	//	return errors.ErrReqParams
	//}
	//
	//needReprovision := req.ExpirationSeconds != nil || req.PType != nil || req.Rules != nil || req.TargetNamespaces != nil
	//if needReprovision {
	//	merged := c.mergePermissionRequest(object, req)
	//	kubeConfig, saNs, saName, err := c.provisionPermission(ctx, merged)
	//	if err != nil {
	//		return err
	//	}
	//	updates["kube_config"] = kubeConfig
	//	updates["sa_name"] = saName
	//	updates["sa_namespace"] = saNs
	//}
	//
	//if err = c.factory.Permission().Update(ctx, permissionId, *req.ResourceVersion, updates); err != nil {
	//	if errors.IsNotUpdated(err) {
	//		return errors.ErrRecordNotUpdate
	//	}
	//	if errors.IsUniqueConstraintError(err) {
	//		return fmt.Errorf("同一用户在该集群下已存在同名授权")
	//	}
	//	klog.Errorf("failed to update permission(%d): %v", permissionId, err)
	//	return errors.ErrInternal
	//}
	return nil
}

// DeletePermission
// 1. 删除数据库记录
// 2. 移除 k8s 的资源
// 3. 删除已授权的集群记录
func (c *cluster) DeletePermission(ctx context.Context, permissionId int64) error {
	object, err := c.factory.Permission().Delete(ctx, permissionId)
	if err != nil {
		klog.Errorf("failed to delete permission(%d): %v", permissionId, err)
		return errors.ErrInternal
	}
	if object == nil {
		return errors.ErrPermissionNotFound
	}

	// 删除 k8s 资源规则
	if err = c.deleteKubernetesRule(ctx, object); err != nil {
		klog.Errorf("清理k8s规则失败 %v", err)
	}

	return c.Delete(ctx, object.ClusterId, true)
}

func (c *cluster) mergePermissionRequest(object *model.Permission, req *types.UpdatePermissionRequest) *types.CreatePermissionRequest {
	merged := &types.CreatePermissionRequest{
		UserId:            object.UserId,
		Name:              object.Name,
		ExpirationSeconds: object.ExpirationSeconds,
		Description:       object.Description,
		PType:             int(object.PType),
		TargetNamespaces:  decodeStringSlice(object.TargetNamespaces),
		Rules:             decodeRules(object.Rules),
		SAName:            object.SAName,
		SANamespace:       object.SANamespace,
	}
	if req.Name != nil {
		merged.Name = *req.Name
	}
	if req.ExpirationSeconds != nil {
		merged.ExpirationSeconds = *req.ExpirationSeconds
	}
	if req.Description != nil {
		merged.Description = *req.Description
	}
	if req.PType != nil {
		merged.PType = *req.PType
	}
	if req.Rules != nil {
		merged.Rules = req.Rules
	}
	if req.TargetNamespaces != nil {
		merged.TargetNamespaces = req.TargetNamespaces
	}
	return merged
}

func (c *cluster) addKubernetesRule(ctx context.Context, req *types.CreatePermissionRequest, clusterSet client.ClusterSet, saNs, saName string) (string, error) {
	kubeClient := clusterSet.Client
	if err := ensureServiceAccount(ctx, kubeClient, saNs, saName); err != nil {
		return "", fmt.Errorf("创建 SA 失败: %w", err)
	}

	crName, err := createClusterRole(ctx, kubeClient, req)
	if err != nil {
		return "", err
	}

	if req.PType == 0 || req.PType == 2 {
		if err = createClusterRoleBinding(ctx, kubeClient, saNs, saName, crName); err != nil {
			return "", err
		}
	} else {
		for _, ns := range req.TargetNamespaces {
			if err = createRoleBinding(ctx, kubeClient, ns, saNs, saName, crName); err != nil {
				klog.Errorf("%v", err)
				return "", err
			}
		}
	}

	expiration := defaultExpiration
	if req.ExpirationSeconds > 0 {
		expiration = time.Duration(req.ExpirationSeconds) * time.Second
	}
	token, err := createToken(ctx, kubeClient, saNs, saName, expiration)
	if err != nil {
		return "", fmt.Errorf("创建 token 失败: %w", err)
	}

	var caData []byte
	if clusterSet.Config.TLSClientConfig.CAData != nil {
		caData = clusterSet.Config.TLSClientConfig.CAData
	}
	kubeConfig := buildKubeconfig(req.Name, clusterSet.Config.Host, caData, token)
	return kubeConfig, nil
}

func (c *cluster) deleteKubernetesRule(ctx context.Context, object *model.Permission) error {
	// TODO: 直接持久化？
	slaveClu, err := c.factory.Cluster().Get(ctx, object.ClusterId)
	if err != nil {
		klog.Errorf("获取从集群(%d)属性失败 %v", object.ClusterId, err)
		return err
	}
	masterClu, err := c.factory.Cluster().Get(ctx, slaveClu.OwnerReference)
	if err != nil {
		klog.Errorf("获取主集群(%d)属性失败 %v", object.ClusterId, err)
		return err
	}

	clusterSet, err := c.GetClusterSetByName(ctx, masterClu.Name)
	if err != nil {
		return fmt.Errorf("获取集群 %s 配置失败: %w", masterClu.Name, err)
	}
	clientSet := clusterSet.Client

	userId := object.UserId
	saName := object.SAName
	saNamespace := object.SANamespace

	// 执行 k8s 资源删除
	// 1. 删除 SA
	_ = clientSet.CoreV1().ServiceAccounts(saNamespace).Delete(ctx, saName, metav1.DeleteOptions{})

	if object.PType == 1 {
		// 2. 删除 ClusterRoleBinding 自定义的类型需要删除
		name := fmt.Sprintf("pixiu-cr-%d", userId)
		klog.Infof("正在删除 cr(%s)", name)
		if err = clientSet.RbacV1().ClusterRoles().Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
			klog.Errorf("删除 cr(%s) 失败 %v", name, err)
		}

		// 3. 删除各命名空间的 ClusterRoleBinding
		bindingName := fmt.Sprintf("pixiu-binding-%s", saName)
		for _, targetNamespace := range decodeStringSlice(object.TargetNamespaces) {
			klog.Infof("正在删除 RoleBindings %s(%s)", targetNamespace, bindingName)
			if err = clientSet.RbacV1().RoleBindings(targetNamespace).Delete(ctx, bindingName, metav1.DeleteOptions{}); err != nil {
				klog.Errorf("删除 ns(%s) RoleBindings(%s) 失败 %v", targetNamespace, bindingName, err)
			}
		}
	}

	return nil
}

func (c *cluster) permissionModel2Type(o *model.Permission) *types.Permission {
	return &types.Permission{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
		UserId:            o.UserId,
		Name:              o.Name,
		ExpirationSeconds: o.ExpirationSeconds,
		PType:             int(o.PType),
		Rules:             decodeRules(o.Rules),
		SAName:            o.SAName,
		SANamespace:       o.SANamespace,
		ClusterId:         o.ClusterId,
		TargetNamespaces:  decodeStringSlice(o.TargetNamespaces),
		Description:       o.Description,
	}
}

func encodeRules(rules []rbacv1.PolicyRule) (string, error) {
	if len(rules) == 0 {
		return "", nil
	}
	b, err := json.Marshal(rules)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func encodeStringSlice(items []string) (string, error) {
	if len(items) == 0 {
		return "", nil
	}
	b, err := json.Marshal(items)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func decodeRules(raw string) []rbacv1.PolicyRule {
	if raw == "" {
		return nil
	}
	var rules []rbacv1.PolicyRule
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return nil
	}
	return rules
}

func decodeStringSlice(raw string) []string {
	if raw == "" {
		return nil
	}
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}
	return items
}

// 创建 ClusterRoleBinding（将 ClusterRole 绑定到 ServiceAccount，集群级）
func createClusterRoleBinding(ctx context.Context, clientSet *kubernetes.Clientset, saNamespace, saName, clusterRoleName string) error {
	bindingName := fmt.Sprintf("pixiu-crb-%s", saName)
	_, err := clientSet.RbacV1().ClusterRoleBindings().Get(ctx, bindingName, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("检查 ClusterRoleBinding 失败: %v", err)
	}

	binding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: bindingName,
			Labels: map[string]string{
				"maintainer": "pixiu",
			},
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      saName,
			Namespace: saNamespace,
		}},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	_, err = clientSet.RbacV1().ClusterRoleBindings().Create(ctx, binding, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("创建 ClusterRoleBinding 失败: %v", err)
	}
	return nil
}

// 创建 RoleBinding（将 ClusterRole 绑定到 ServiceAccount，限定在指定命名空间）
func createRoleBinding(ctx context.Context, clientSet *kubernetes.Clientset, namespace, saNamespace, saName, clusterRoleName string) error {
	bindingName := fmt.Sprintf("pixiu-binding-%s", saName)
	_, err := clientSet.RbacV1().RoleBindings(namespace).Get(ctx, bindingName, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("检查 RoleBinding 失败: %v", err)
	}

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: bindingName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: saNamespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	_, err = clientSet.RbacV1().RoleBindings(namespace).Create(ctx, roleBinding, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("创建 RoleBinding 失败: %v", err)
	}
	return nil
}

// 创建 ClusterRole（如果已存在则跳过）
func createClusterRole(ctx context.Context, clientSet *kubernetes.Clientset, req *types.CreatePermissionRequest) (string, error) {
	if req.PType == 0 {
		return "view", nil
	}
	if req.PType == 2 {
		return "cluster-admin", nil
	}

	name := fmt.Sprintf("pixiu-cr-%d", req.UserId)
	_, err := clientSet.RbacV1().ClusterRoles().Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return name, nil
	}

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"maintainer": "pixiu",
			},
		},
		Rules: req.Rules,
	}
	_, err = clientSet.RbacV1().ClusterRoles().Create(ctx, clusterRole, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("创建 ClusterRole 失败: %v", err)
	}
	return name, nil
}

func ensureServiceAccount(ctx context.Context, client kubernetes.Interface, ns, name string) error {
	saClient := client.CoreV1().ServiceAccounts(ns)
	_, err := saClient.Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}
	_, err = saClient.Create(ctx, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				"maintainer": "pixiu",
			}},
	}, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func createToken(ctx context.Context, client kubernetes.Interface, ns, name string, duration time.Duration) (string, error) {
	tr, err := client.CoreV1().ServiceAccounts(ns).CreateToken(ctx, name,
		&authenticationv1.TokenRequest{
			Spec: authenticationv1.TokenRequestSpec{
				ExpirationSeconds: ptrInt64(int64(duration.Seconds())),
			},
		}, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}
	return tr.Status.Token, nil
}

func buildKubeconfig(name, server string, caData []byte, token string) string {
	config := clientcmdapi.Config{
		Kind:       "Config",
		APIVersion: "v1",
		Clusters: map[string]*clientcmdapi.Cluster{
			name: {
				Server:                   server,
				CertificateAuthorityData: caData,
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			name: {Token: token},
		},
		Contexts: map[string]*clientcmdapi.Context{
			name: {Cluster: name, AuthInfo: name},
		},
		CurrentContext: name,
	}
	if len(caData) == 0 {
		config.Clusters[name].InsecureSkipTLSVerify = true
	}

	b, _ := clientcmd.Write(config)
	return base64.StdEncoding.EncodeToString(b)
}

func ptrInt64(v int64) *int64 { return &v }
