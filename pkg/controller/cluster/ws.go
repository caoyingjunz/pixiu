/*
Copyright 2021 The Pixiu Authors.

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

package cluster

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/client"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/gorilla/websocket"
	appv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/types"
	sshutil "github.com/caoyingjunz/pixiu/pkg/util/ssh"
)

func (c *cluster) WsPodHandler(ctx context.Context, opt *types.WebShellOptions, w http.ResponseWriter, r *http.Request) error {
	cs, err := c.GetClusterSetByName(ctx, opt.Cluster)
	if err != nil {
		klog.Errorf("failed to get cluster(%s) client set: %v", opt.Cluster, err)
		return err
	}

	session, err := types.NewTerminalSession(w, r)
	if err != nil {
		return err
	}
	// 处理关闭
	defer func() {
		_ = session.Close()
	}()
	klog.Infof("connecting to %s/%s,", opt.Namespace, opt.Pod)

	cmd := opt.Command
	if len(cmd) == 0 {
		cmd = "/bin/bash"
	}
	execCommand := []string{cmd}
	if len(opt.CommandArgs) > 0 {
		execCommand = opt.CommandArgs
	}

	// 组装 POST 请求
	req := cs.Client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(opt.Pod).
		Namespace(opt.Namespace).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Container: opt.Container,
			Command:   execCommand,
			Stderr:    true,
			Stdin:     true,
			Stdout:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

	// remotecommand 主要实现了http 转 SPDY 添加X-Stream-Protocol-Version相关header 并发送请求
	// 使用 client.NewSPDYExecutor，确保隧道模式下走 rest.Config.Dial
	executor, err := client.NewSPDYExecutor(cs.Config, "POST", req.URL())
	if err != nil {
		return err
	}
	// 与 kubelet 建立 stream 连接
	if err = executor.Stream(remotecommand.StreamOptions{
		Stdout:            session,
		Stdin:             session,
		Stderr:            session,
		TerminalSizeQueue: session,
		Tty:               true,
	}); err != nil {
		_, _ = session.Write([]byte("exec pod command failed," + err.Error()))
		// 标记关闭terminal
		session.Done()
	}

	return nil
}

var BufPool = sync.Pool{New: func() interface{} { return new(bytes.Buffer) }}

func (c *cluster) WsNodeHandler(ctx context.Context, req types.WebSSHRequest, w http.ResponseWriter, r *http.Request) error {
	sshConfig, err := c.ResolveSSHConfigForHost(ctx, req.Host)
	if err != nil {
		return err
	}

	upgrader := &websocket.Upgrader{
		ReadBufferSize:   1024,
		WriteBufferSize:  1024 * 10,
		HandshakeTimeout: time.Second * 2,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		Subprotocols: []string{r.Header.Get("Sec-WebSocket-Protocol")},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// 升级失败时尚未 hijack，可交给上层返回 JSON 错误
		return err
	}
	defer conn.Close()

	// Upgrade 成功后 net/http 连接已被 hijack，禁止再向 ResponseWriter 写 JSON（否则会 panic:
	// http: connection has been hijacked）。后续错误仅记录日志并通过 WebSocket 提示客户端。
	sshClient, err := sshutil.NewSSHClient(sshConfig)
	if err != nil {
		klog.Errorf("node ssh dial failed (host=%s): %v", sshConfig.Host, err)
		_ = conn.WriteMessage(websocket.TextMessage, []byte("\r\n[SSH 连接失败] "+err.Error()+"\r\n"))
		return nil
	}
	defer sshClient.Close()

	turn, err := types.NewTurn(conn, sshClient)
	if err != nil {
		klog.Errorf("node ssh session failed (host=%s): %v", sshConfig.Host, err)
		_ = conn.WriteMessage(websocket.TextMessage, []byte("\r\n[SSH 会话建立失败] "+err.Error()+"\r\n"))
		return nil
	}
	defer turn.Close()

	// 处理连接
	handler(turn)

	return nil
}

func handler(turn *types.Turn) {
	logBuff := BufPool.Get().(*bytes.Buffer)
	logBuff.Reset()
	defer BufPool.Put(logBuff)

	wg := &sync.WaitGroup{}
	wg.Add(2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go turn.StartLoopRead(ctx, wg, logBuff)
	go turn.StartSessionWait(wg)

	wg.Wait()
}

// WsClusterHandler 连接 k8s 集群的
// 1. 启动运行 pod，并挂载 kubeconfig，工作空间是 pixiu-system，必须确保 pixiu-system 存在
// 2. 等待 pod running
// 3. ws pod
func (c *cluster) WsClusterHandler(ctx context.Context, req types.ClusterWebRequest, w http.ResponseWriter, r *http.Request) error {
	user, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		klog.Errorf("failed to get user from context: %v", err)
		return err
	}

	ownerClusterName, err := c.getOwnerClusterName(ctx, req.ClusterId)
	if err != nil {
		klog.Errorf("failed to get owner reference cluster name: %v", err)
		return err
	}
	clientSet, err := c.GetClusterSetByName(ctx, ownerClusterName)
	if err != nil {
		klog.Errorf("failed to get cluster(%s) client set: %v", req.ClusterName, err)
		return err
	}

	// 隧道导入时可能未创建 pixiu-system，cloudShell 启动前幂等补齐
	if err = c.ensurePixiuSystemNamespace(ctx, &clientSet); err != nil {
		klog.Errorf("failed to ensure namespace %s for CloudShell: %v", pixiuSystemNamespace, err)
		return err
	}

	stsName := fmt.Sprintf("ws-%d-%d", req.ClusterId, user.Id)
	namespace := pixiuSystemNamespace
	podName := stsName + "-0"

	_, err = clientSet.Client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err == nil {
		klog.Infof("pod(%s) already exists, reuse it", podName)
	} else {
		if errors.IsNotFound(err) {
			if err = c.CreateAndWaitForPodRunning(ctx, clientSet, req, user); err != nil {
				return err
			}
		} else {
			klog.Errorf("failed to get pod(%s): %v", podName, err)
			return err
		}
	}

	// 会话结束后回收 StatefulSet（级联删除 pod），下次打开会重新创建
	defer func() {
		if delErr := clientSet.Client.AppsV1().StatefulSets(namespace).Delete(ctx, stsName, metav1.DeleteOptions{}); delErr != nil && !errors.IsNotFound(delErr) {
			klog.Errorf("failed to delete CloudShell sts(%s/%s): %v", namespace, stsName, delErr)
		} else {
			klog.Infof("CloudShell session closed, deleted sts(%s/%s)", namespace, stsName)
		}
	}()

	return c.WsPodHandler(ctx, &types.WebShellOptions{
		Cluster:     ownerClusterName,
		Namespace:   namespace,
		Pod:         podName,
		Container:   "pixiu-ws-toolbox",
		CommandArgs: cloudShellBashCommand(),
	}, w, r)
}

// cloudShellBashCommand 启动带彩色提示符的交互 bash，风格参考云厂商 CloudShell。
func cloudShellBashCommand() []string {
	// [user@host path]# — 浅色背景下使用偏深的 ANSI 色，避免过亮刺眼
	const ps1 = `[\[\033[00;32m\]\u\[\033[00m\]@\[\033[00;36m\]\h\[\033[00m\] \[\033[00;33m\]\w\[\033[00m\]]\[\033[00;35m\]# \[\033[00m\]`
	return []string{
		"/bin/bash",
		"-c",
		"export PS1='" + ps1 + "'; cd ~ 2>/dev/null || cd /root; exec /bin/bash -i",
	}
}

// cloudShellAdminSA 是 root/owner（直接访问主集群）时 cloudShell pod 使用的 ServiceAccount，
// 幂等创建并绑定内置 cluster-admin。
const cloudShellAdminSA = "pixiu-cloudshell-admin"

// cloudShellKubeServer cloudShell pod 内 kubeconfig 使用的集群内 server 地址（pod 只能经集群内地址访问 apiserver）。
const cloudShellKubeServer = "https://kubernetes.default.svc"

// resolveCloudShellServiceAccount 确定 cloudShell pod 使用的 ServiceAccount：
//   - credCluster.PermissionId == 0（root/owner 直接访问主集群）：使用/幂等创建 admin SA；
//   - credCluster.PermissionId != 0（readonly/custom 回落其授权子集群）：使用该子集群授权记录对应 SA。
//
// 命名空间必须为 pixiu-system（cloudShell pod 所在 namespace，pod 只能用同 namespace 的 SA）。
func (c *cluster) resolveCloudShellServiceAccount(ctx context.Context, clientSet *client.ClusterSet, credCluster *model.Cluster) (string, error) {
	if credCluster.PermissionId == 0 {
		if err := ensureCloudShellAdminSA(ctx, clientSet); err != nil {
			return "", err
		}
		return cloudShellAdminSA, nil
	}

	// 子集群行（PermissionId != 0 且 OwnerReference=master）的 PermissionId 即其关联授权记录 id，取其 SAName
	p, err := c.factory.Permission().Get(ctx, credCluster.PermissionId)
	if err != nil {
		return "", fmt.Errorf("failed to get permission(%d) for cloudshell SA: %w", credCluster.PermissionId, err)
	}
	if p == nil || p.SAName == "" {
		return "", fmt.Errorf("permission(%d) has no SAName, cannot resolve cloudshell SA", credCluster.PermissionId)
	}
	if p.SANamespace != "" && p.SANamespace != pixiuSystemNamespace {
		klog.Warningf("permission(%d) SANamespace(%s) != pixiu-system, force to pixiu-system for cloudshell", p.Id, p.SANamespace)
	}
	return p.SAName, nil
}

// ensureCloudShellAdminSA 幂等创建 cloudShell admin SA 并绑定内置 cluster-admin（root/owner 直接访问主集群场景）。
func ensureCloudShellAdminSA(ctx context.Context, clientSet *client.ClusterSet) error {
	kubeClient := clientSet.Client
	if err := ensureServiceAccountByName(ctx, kubeClient, pixiuSystemNamespace, cloudShellAdminSA); err != nil {
		return err
	}

	bindingName := "pixiu-cloudshell-admin-binding"
	desired := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   bindingName,
			Labels: map[string]string{"maintainer": "pixiu"},
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      cloudShellAdminSA,
			Namespace: pixiuSystemNamespace,
		}},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	existing, err := kubeClient.RbacV1().ClusterRoleBindings().Get(ctx, bindingName, metav1.GetOptions{})
	if err == nil {
		// 完整对比 Subjects 与 RoleRef（含 Kind/APIGroup），一致则不触发 Update，避免多余 API 调用
		if apiequality.Semantic.DeepEqual(existing.Subjects, desired.Subjects) &&
			apiequality.Semantic.DeepEqual(existing.RoleRef, desired.RoleRef) {
			return nil
		}
		// 绑定被外部改动，幂等修复
		existing.Subjects = desired.Subjects
		existing.RoleRef = desired.RoleRef
		if _, err = kubeClient.RbacV1().ClusterRoleBindings().Update(ctx, existing, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to update ClusterRoleBinding(%s): %w", bindingName, err)
		}
		return nil
	}
	if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get ClusterRoleBinding(%s): %w", bindingName, err)
	}
	if _, err = kubeClient.RbacV1().ClusterRoleBindings().Create(ctx, desired, metav1.CreateOptions{}); err != nil {
		if errors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("failed to create ClusterRoleBinding(%s): %w", bindingName, err)
	}
	return nil
}

// ensureServiceAccountByName 幂等创建指定 ServiceAccount（已存在则复用）。
func ensureServiceAccountByName(ctx context.Context, kubeClient kubernetes.Interface, ns, name string) error {
	_, err := kubeClient.CoreV1().ServiceAccounts(ns).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}
	_, err = kubeClient.CoreV1().ServiceAccounts(ns).Create(ctx, &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    map[string]string{"maintainer": "pixiu"},
		},
	}, metav1.CreateOptions{})
	if errors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func (c *cluster) CreateAndWaitForPodRunning(ctx context.Context, clientSet client.ClusterSet, req types.ClusterWebRequest, user *model.User) error {
	stsName := fmt.Sprintf("ws-%d-%d", req.ClusterId, user.Id)
	podName := stsName + "-0"
	namespace := pixiuSystemNamespace
	labels := map[string]string{"maintainer": "pixiu", "cluster": req.ClusterName, "app": stsName}

	// 凭据降权：按角色回落 scoped 凭据，禁止把 admin 私钥挂载给 readonly/custom 用户。
	// root/owner 拿到主集群 admin 属其本身权限，可接受；readonly/custom 回落其子集群 scoped 凭据。
	ownerClusterName, err := c.getOwnerClusterName(ctx, req.ClusterId)
	if err != nil {
		return err
	}
	credCluster, err := c.AuthorizeClusterAccessByName(ctx, user, ownerClusterName)
	if err != nil {
		klog.Errorf("failed to authorize cluster(%s) access for user(%d): %v", ownerClusterName, user.Id, err)
		return err
	}

	// 不给 pod 注入任何明文凭据（无 env / 无 kubeconfig 注入）。
	// 由 initContainer 从自动挂载的 ServiceAccount token + ca.crt 自生成 kubeconfig 写入 emptyDir，
	// 主容器挂载到 /root/.kube/config。SA token 为 projected token（约 1 小时轮换），pod 关闭即消失。
	saName, err := c.resolveCloudShellServiceAccount(ctx, &clientSet, credCluster)
	if err != nil {
		klog.Errorf("failed to resolve CloudShell serviceAccount for user(%d): %v", user.Id, err)
		return err
	}

	// 创建 sts
	kubeConfigVolumeName := "kubeconfig"
	kubeConfigMountPath := "/root/.kube/config"
	kubeConfigDirMountPath := "/cloudshell-kube"
	// initContainer 读取 SA 目录生成 kubeconfig 的脚本：server 用集群内地址，证书与 token 均来自自动挂载的 SA 目录
	kubeconfigInitScript := `set -e
TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)
CA_B64=$(cat /var/run/secrets/kubernetes.io/serviceaccount/ca.crt | base64 | tr -d '\n')
cat > ` + kubeConfigDirMountPath + `/kubeconfig <<PIXIU_KUBECONFIG
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: ${CA_B64}
    server: ` + cloudShellKubeServer + `
  name: cloudshell
contexts:
- context:
    cluster: cloudshell
    user: cloudshell
  name: cloudshell
current-context: cloudshell
users:
- name: cloudshell
  user:
    token: ${TOKEN}
PIXIU_KUBECONFIG
chmod 0600 ` + kubeConfigDirMountPath + `/kubeconfig
`
	_, err = clientSet.Client.AppsV1().StatefulSets(namespace).Create(ctx, &appv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:   stsName,
			Labels: labels,
		},
		Spec: appv1.StatefulSetSpec{
			ServiceName: stsName,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: v1.PodSpec{
					ServiceAccountName: saName,
					InitContainers: []v1.Container{
						{
							Name:            "kubeconfig-init",
							Image:           c.cc.Default.Toolbox,
							ImagePullPolicy: "IfNotPresent",
							Command: []string{
								"/bin/sh",
								"-c",
								kubeconfigInitScript,
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      kubeConfigVolumeName,
									MountPath: kubeConfigDirMountPath,
								},
							},
						},
					},
					Containers: []v1.Container{
						{
							Name:            "pixiu-ws-toolbox",
							Image:           c.cc.Default.Toolbox,
							ImagePullPolicy: "IfNotPresent",
							WorkingDir:      "/root",
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      kubeConfigVolumeName,
									MountPath: kubeConfigMountPath,
									SubPath:   "kubeconfig",
								},
							},
						},
					},
					Volumes: []v1.Volume{
						{
							Name: kubeConfigVolumeName,
							VolumeSource: v1.VolumeSource{
								EmptyDir: &v1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			klog.Errorf("failed to create ws-cluster: %v", err)
			return err
		}
		klog.Infof("ws-cluster %s/%s already exists, reuse it", namespace, stsName)
	}

	if err = waitForPodRunning(ctx, clientSet.Client, namespace, podName, 10*time.Minute); err != nil {
		return fmt.Errorf("wait pod %s/%s running: %w", namespace, podName, err)
	}

	return nil
}

// waitForPodRunning 轮询等待 Pod 进入 Running 阶段（STS 默认 Pod 名为 <stsName>-0）
func waitForPodRunning(ctx context.Context, client kubernetes.Interface, namespace, podName string, timeout time.Duration) error {
	return wait.PollImmediateWithContext(ctx, 2*time.Second, timeout, func(ctx context.Context) (bool, error) {
		pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}

		switch pod.Status.Phase {
		case v1.PodRunning:
			return true, nil
		case v1.PodFailed, v1.PodSucceeded:
			return false, fmt.Errorf("pod entered terminal phase %s", pod.Status.Phase)
		default:
			return false, nil
		}
	})
}

func (c *cluster) getOwnerClusterName(ctx context.Context, clusterId int64) (string, error) {
	obj, err := c.factory.Cluster().Get(ctx, clusterId)
	if err != nil {
		return "", err
	}
	if obj == nil {
		return "", fmt.Errorf("cluster %d not found", clusterId)
	}

	// 如果本身就是master集群，直接返回
	if obj.PermissionId == 0 {
		return obj.Name, nil
	}

	masterObj, err := c.factory.Cluster().Get(ctx, obj.OwnerReference)
	if err != nil || masterObj == nil {
		return "", fmt.Errorf("cluster %d not found or get failed", clusterId)
	}
	return masterObj.Name, nil
}
