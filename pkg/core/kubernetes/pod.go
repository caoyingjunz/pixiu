package kubernetes

import (
	"bufio"
	"context"
	"io"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/igm/sockjs-go/v3/sockjs"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/db"
	"github.com/caoyingjunz/gopixiu/pkg/log"
	"github.com/caoyingjunz/gopixiu/pkg/util/cipher"
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

func (c *pods) NewHandler(test *types.Test) error {
	sockjs.NewHandler("/webshell/ws", sockjs.DefaultOptions, func(session sockjs.Session) {
		if err := c.WebShellHandler(&types.WebShell{
			Conn:      session,
			SizeChan:  make(chan *remotecommand.TerminalSize),
			Namespace: test.Namespace,
			Pod:       test.Pod,
			Container: test.Container,
		}, "/bin/bash", test); err != nil {
		}
	}).ServeHTTP(c.Writer, c.Request)
	return nil
}

func (c *pods) WebShellHandler(w *types.WebShell, cmd string, webShellOptions *types.Test) error {

	EncryptKubeConfig, err := c.factory.Cloud().GetByName(context.TODO(), webShellOptions.CloudName)
	if err != nil {
		log.Logger.Errorf("failed to get %s EncryptKubeConfig: %v", webShellOptions.CloudName, err)
	}
	decrypt, err := cipher.Decrypt(EncryptKubeConfig.KubeConfig)
	if err != nil {
		log.Logger.Errorf("failed to get  %s decrypt: %v", EncryptKubeConfig.KubeConfig, err)
	}
	config, err := clientcmd.RESTConfigFromKubeConfig(decrypt)
	if err != nil {
		log.Logger.Errorf("failed to get %s config: %v", decrypt, err)
	}

	req := c.client.RESTClient().Post().
		Resource("pods").
		Name(w.Pod).
		Namespace(w.Namespace).
		SubResource("exec").
		Param("container", w.Container).
		Param("stdin", "true").
		Param("stdout", "true").
		Param("stderr", "true").
		Param("command", cmd).
		Param("tty", "true")
	if err != nil {
		return err
	}
	req.VersionedParams(&v1.PodExecOptions{
		Container: w.Container,
		Command:   []string{},
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
		TTY:       true,
	},
		scheme.ParameterCodec,
	)
	executor, err := remotecommand.NewSPDYExecutor(
		config, http.MethodPost, req.URL(),
	)
	if err != nil {
		return err
	}
	return executor.Stream(remotecommand.StreamOptions{
		Stdin:             w,
		Stdout:            w,
		Stderr:            w,
		Tty:               true,
		TerminalSizeQueue: w,
	})
}
