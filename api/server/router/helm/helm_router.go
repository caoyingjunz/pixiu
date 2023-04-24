package helm

import (
	"fmt"

	"github.com/gin-gonic/gin"
	client "github.com/mittwald/go-helm-client"
	restClient "k8s.io/client-go/rest"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/errors"
	"github.com/caoyingjunz/pixiu/pkg/log"
	"github.com/caoyingjunz/pixiu/pkg/pixiu"
)

func (h helmRouter) ListReleasesHandler(c *gin.Context) {
	resp := httputils.NewResponse()
	var helm Helm
	if err := c.ShouldBindUri(&helm); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	config, exists := pixiu.CoreV1.Cloud().GetClusterConfig(c, helm.CloudName)
	if !exists {
		httputils.SetFailed(c, resp, fmt.Errorf("cluster %q not register", helm.CloudName))
		return
	}
	//TODO  查看获取helm客户端相关逻辑，确定client是否支持缓存
	helmClient, err := getHelmClient(helm, config)
	if err != nil {
		httputils.SetFailed(c, resp, errors.OperateFailed)
		return
	}
	if releases, err := helmClient.ListDeployedReleases(); err != nil {
		httputils.SetFailed(c, resp, errors.OperateFailed)
		return
	} else {
		resp.Result = releases
	}
	httputils.SetSuccess(c, resp)
}

func getHelmClient(helm Helm, config *restClient.Config) (client.Client, error) {
	opt := &client.RestConfClientOptions{
		Options: &client.Options{
			Namespace:        helm.Namespace,
			RepositoryCache:  "/tmp/.helmcache",
			RepositoryConfig: "/tmp/.helmrepo",
			Debug:            true,
			Linting:          false,
			DebugLog: func(format string, v ...interface{}) {
				log.Logger.Infof(format, v)
			},
		},
		RestConfig: config,
	}
	helmClient, err := client.NewClientFromRestConf(opt)
	return helmClient, err
}
