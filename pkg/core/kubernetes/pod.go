package kubernetes

import (
	"bufio"
	"context"
	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/db"
	"github.com/caoyingjunz/gopixiu/pkg/log"
	"github.com/caoyingjunz/gopixiu/pkg/util/cipher"
	"github.com/gorilla/websocket"
	"io"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"net/http"
)

type PodsGetter interface {
	Pods(cloud string) PodInterface
}

type PodInterface interface {
	Logs(ctx context.Context, ws *websocket.Conn, options *types.LogsOptions) error
	NewHandler(webShellOptions *types.Test) error
}

type pods struct {
	client  *kubernetes.Clientset
	cloud   string
	factory db.ShareDaoFactory
	Writer  http.ResponseWriter
	Request *http.Request
}

func NewPods(c *kubernetes.Clientset, cloud string) *pods {
	return &pods{
		client: c,
		cloud:  cloud,
	}
}

func (c *pods) Logs(ctx context.Context, ws *websocket.Conn, options *types.LogsOptions) error {
	opts := &coreV1.PodLogOptions{
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

func (c *pods) NewHandler(test *types.Test) error {
	// 加载ClientSet
	EncryptKubeConfig, err := c.factory.KubeConfig().List(context.TODO(), test.CloudName)
	if err != nil {
		log.Logger.Errorf("failed to get %s EncryptKubeConfig: %v", test.CloudName, err)
	}
	for _, config := range EncryptKubeConfig {
		decrypt, err := cipher.Decrypt(config.Config)
		if err != nil {
			log.Logger.Errorf("failed to get  %s decrypt: %v", config.Config, err)
		}
		newconfig, err := clientcmd.RESTConfigFromKubeConfig(decrypt)
		if err != nil {
			log.Logger.Errorf("failed to get %s config: %v", decrypt, err)
		}
		// new一个TerminalSession类型的pty实例
		pty, err := types.NewTerminalSession(c.Writer, c.Request, nil)
		if err != nil {
			return err
		}
		// 处理关闭
		defer func() {
			pty.Close()
		}()
		// 组装POST请求
		req := c.client.CoreV1().RESTClient().Post().
			Resource("pods").
			Name(test.Pod).
			Namespace(namespace).
			SubResource("exec").
			VersionedParams(&coreV1.PodExecOptions{
				Container: test.Container,
				Command:   []string{"/bin/bash"},
				Stderr:    true,
				Stdin:     true,
				Stdout:    true,
				TTY:       true,
			}, scheme.ParameterCodec)
		// remotecommand 主要实现了http 转 SPDY 添加X-Stream-Protocol-Version相关header 并发送请求
		executor, err := remotecommand.NewSPDYExecutor(newconfig, "POST", req.URL())
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
	}
	return nil
}
