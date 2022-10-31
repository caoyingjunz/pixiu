package kubernetes

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"path/filepath"

	"github.com/gorilla/websocket"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/homedir"

	"github.com/caoyingjunz/gopixiu/api/types"
)

type PodsGetter interface {
	Pods(cloud string) PodInterface
}

type PodInterface interface {
	Logs(ctx context.Context, ws *websocket.Conn, options *types.LogsOptions) error

	NewWebShellHandler(webShellOptions *types.Test, w http.ResponseWriter, r *http.Request) error
}

type pods struct {
	client *kubernetes.Clientset
	cloud  string
}

func NewPods(c *kubernetes.Clientset, cloud string) *pods {
	return &pods{
		client: c,
		cloud:  cloud,
	}
}

func (c *pods) Logs(ctx context.Context, ws *websocket.Conn, options *types.LogsOptions) error {
	opts := &v1.PodLogOptions{
		Follow:    true,
		Container: options.ContainerName,
	}

	request := c.client.CoreV1().Pods(options.Namespace).GetLogs(options.ObjectName, opts)
	readCloser, err := request.Stream(context.TODO())
	if err != nil {
		return err
	}
	defer readCloser.Close()

	r := bufio.NewReader(readCloser)
	for {
		bytes, err := r.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				return err
			}
			return nil
		}

		// 输出
		ws.WriteMessage(websocket.TextMessage, bytes)
	}
}

// TODO: 后续优化
func (c *pods) NewWebShellHandler(test *types.Test, w http.ResponseWriter, r *http.Request) error {
	// 加载 ClientSet
	config, err := clientcmd.BuildConfigFromFlags("", filepath.Join(homedir.HomeDir(), ".kube", "config"))
	if err != nil {
		return err
	}
	ClientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	// new一个TerminalSession类型的pty实例
	pty, err := types.NewTerminalSession(w, r, nil)
	if err != nil {
		return err
	}
	// 处理关闭
	defer func() {
		pty.Close()
	}()

	// 组装POST请求
	req := ClientSet.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(test.Pod).
		Namespace(test.Namespace).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Container: test.Container,
			Command:   []string{"/bin/bash"},
			Stderr:    true,
			Stdin:     true,
			Stdout:    true,
			TTY:       true,
		}, scheme.ParameterCodec)
	// remotecommand 主要实现了http 转 SPDY 添加X-Stream-Protocol-Version相关header 并发送请求
	executor, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return err
	}
	// 与 kubelet 建立 stream 连接
	err = executor.Stream(remotecommand.StreamOptions{
		Stdout:            pty,
		Stdin:             pty,
		Stderr:            pty,
		TerminalSizeQueue: pty,
		Tty:               true,
	})
	if err != nil {
		// 将报错返回给web端
		pty.Write([]byte("exec pod command failed," + err.Error()))
		// 标记关闭terminal
		pty.Done()
	}

	return nil
}
