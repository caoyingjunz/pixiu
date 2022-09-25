package kubernetes

import (
	"bufio"
	"context"
	"io"

	"github.com/gorilla/websocket"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/caoyingjunz/gopixiu/api/types"
)

type PodsGetter interface {
	Pods(cloud string) PodInterface
}

type PodInterface interface {
	Logs(ctx context.Context, ws *websocket.Conn, options *types.LogsOptions) error
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
