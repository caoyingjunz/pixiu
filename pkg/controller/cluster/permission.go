package cluster

import (
	"context"
	"fmt"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/caoyingjunz/pixiu/pkg/types"
)

const (
	defaultExpiration = 365 * 24 * time.Hour
)

// CreateKubeConfig 创建 scoped kubeconfig
func (c *cluster) CreateKubeConfig(ctx context.Context, req *types.CreateKubeConfigRequest) error {
	clusterSet, err := c.GetClusterSetByName(ctx, req.Cluster)
	if err != nil {
		return fmt.Errorf("获取集群 %s 配置失败: %w", req.Cluster, err)
	}
	kubeClient := clusterSet.Client

	saNs := req.SANamespace
	if len(saNs) == 0 {
		saNs = "pixiu-system"
	}
	saName := req.SAName
	if len(saName) == 0 {
		saName = fmt.Sprintf("pixiu-sa-%d", req.UserId)
	}

	// 确保 SA 存在
	if err = ensureServiceAccount(ctx, kubeClient, saNs, saName); err != nil {
		return fmt.Errorf("创建 SA 失败: %w", err)
	}

	// 确保 cluster role 存在
	crName, err := createClusterRole(ctx, kubeClient, req)
	if err != nil {
		return err
	}

	// 关联
	if req.PType == 0 || req.PType == 2 {
		// TODO
	} else {
		for _, ns := range req.TargetNamespaces {
			if err = createRoleBinding(ctx, kubeClient, ns, saNs, saName, crName); err != nil {
				return err
			}
		}
	}

	// 3. 创建 token
	expiration := defaultExpiration
	if req.ExpirationSeconds > 0 {
		expiration = time.Duration(req.ExpirationSeconds) * time.Second
	}
	token, err := createToken(ctx, kubeClient, saNs, saName, expiration)
	if err != nil {
		return fmt.Errorf("创建 token 失败: %w", err)
	}

	// 4. 组装 kubeconfig
	// 组装 kubeconfig
	var caData []byte
	if clusterSet.Config.TLSClientConfig.CAData != nil {
		caData = clusterSet.Config.TLSClientConfig.CAData
	}
	_ = buildKubeconfig(req.Name, clusterSet.Config.Host, caData, token)
	return nil
}

type clusterRoleSpec struct {
	apiGroup  string
	resources []string
	verbs     []string
}

// 创建 RoleBinding（将 ClusterRole 绑定到 ServiceAccount，限定在指定命名空间）
func createRoleBinding(ctx context.Context, clientSet *kubernetes.Clientset, namespace, saNamespace, saName, clusterRoleName string) error {
	bindingName := fmt.Sprintf("%s-binding", saName)
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
func createClusterRole(ctx context.Context, clientSet *kubernetes.Clientset, req *types.CreateKubeConfigRequest) (string, error) {
	// 只读
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
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
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
	clientcmd.WriteToFile(config, "test.kubeconfig")

	return string(b)
}

func ptrInt64(v int64) *int64 { return &v }
