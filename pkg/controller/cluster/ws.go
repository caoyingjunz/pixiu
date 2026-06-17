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

	"github.com/caoyingjunz/pixiu/pkg/client"
	"github.com/gorilla/websocket"
	appv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
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

	// 组装 POST 请求
	req := cs.Client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(opt.Pod).
		Namespace(opt.Namespace).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Container: opt.Container,
			Command:   []string{cmd},
			Stderr:    true,
			Stdin:     true,
			Stdout:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

	// remotecommand 主要实现了http 转 SPDY 添加X-Stream-Protocol-Version相关header 并发送请求
	executor, err := remotecommand.NewSPDYExecutor(cs.Config, "POST", req.URL())
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

	stsName := fmt.Sprintf("ws-%d-%d", req.ClusterId, req.UserId)
	namespace := "pixiu-system" // 导入集群或者部署集群的时候已确保存在
	podName := stsName + "-0"
	rsync2.sh
	_, err = clientSet.Client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err == nil {
		klog.Infof("pod(%s) already exists, reuse it", podName)
	} else {
		if errors.IsNotFound(err) {
			if err = c.CreateAndWaitForPodRunning(ctx, clientSet, req); err != nil {
				return err
			}
		} else {
			klog.Errorf("failed to get pod(%s): %v", podName, err)
			return err
		}
	}

	return c.WsPodHandler(ctx, &types.WebShellOptions{
		Cluster:   ownerClusterName,
		Namespace: namespace,
		Pod:       podName,
		Container: "pixiu-ws-toolbox",
		Command:   "/bin/bash",
	}, w, r)
}

func (c *cluster) CreateAndWaitForPodRunning(ctx context.Context, clientSet client.ClusterSet, req types.ClusterWebRequest) error {
	stsName := fmt.Sprintf("ws-%d-%d", req.ClusterId, req.UserId)
	podName := stsName + "-0"
	namespace := "pixiu-system" // 导入集群或者部署集群的时候已确保存在
	cmName := fmt.Sprintf("ws-%d-%d", req.ClusterId, req.UserId)
	labels := map[string]string{"maintainer": "pixiu", "cluster": req.ClusterName, "app": stsName}

	// 创建 cm，创建 sts，等待pod启动
	// 获取 kubeconfig 配置
	obj, err := c.factory.Cluster().Get(ctx, req.ClusterId)
	if err != nil {
		return err
	}
	kubeConfigBytes, err := client.ParseKubeConfigBytes(obj.KubeConfig)
	if err != nil {
		return err
	}
	_, err = clientSet.Client.CoreV1().ConfigMaps(namespace).Create(ctx, &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:   cmName,
			Labels: labels,
		},
		Data: map[string]string{"kubeconfig": string(kubeConfigBytes)},
	}, metav1.CreateOptions{})
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
		klog.Infof("ws-cluster %s/%s already exists, reuse it", namespace, cmName)
	}

	// 创建 sts
	kubeConfigVolumeName := "kubeconfig"
	kubeConfigMountPath := "/root/.kube/config"
	defaultMode := int32(0600)
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
								ConfigMap: &v1.ConfigMapVolumeSource{
									LocalObjectReference: v1.LocalObjectReference{Name: cmName},
									DefaultMode:          &defaultMode,
								},
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
