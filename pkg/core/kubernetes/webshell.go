package kubernetes

import (
	"context"
	"net/http"

	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/core/client"
	"github.com/caoyingjunz/gopixiu/pkg/db"
	"github.com/caoyingjunz/gopixiu/pkg/log"
	"github.com/caoyingjunz/gopixiu/pkg/util/cipher"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

var clientSets client.ClientsInterface

type WebShellGetter interface {
	WebShell(cloud string) WebShellInterface
}

type WebShellInterface interface {
	WebShellHandler(w *types.WebShell, cmd string, webShellOptions types.WebShellOptions) error
}

type webShell struct {
	client  *kubernetes.Clientset
	cloud   string
	factory db.ShareDaoFactory
}

func NewWebShell(c *kubernetes.Clientset, cloud string) *webShell {
	return &webShell{
		client: c,
		cloud:  cloud,
	}
}

func (c *webShell) WebShellHandler(w *types.WebShell, cmd string, webShellOptions types.WebShellOptions) error {

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
