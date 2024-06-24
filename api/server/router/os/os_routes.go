package os

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
)

type OsMeta struct {
	Centos []string `json:"centos,omitempty"`
	Debian []string `json:"debian,omitempty"`
	Ubuntu []string `json:"ubuntu,omitempty"`
}

func (or *OSRouter) getOsList(c *gin.Context) {
	r := httputils.NewResponse()

	switch c.Query("name") {
	case "centos":
		obj := &OsMeta{
			Centos: []string{"centos7", "centos8", "centos9"},
		}
		r.Result = obj
	case "ubuntu":
		obj := &OsMeta{
			Ubuntu: []string{"ubuntu18.04", "ubuntu20.04"},
		}
		r.Result = obj
	case "debian":
		obj := &OsMeta{
			Debian: []string{"debian9", "debian10"},
		}

		r.Result = obj
	default:
		obj := &OsMeta{
			Centos: []string{"centos7", "centos8", "centos9"},
			Ubuntu: []string{"ubuntu18.04", "ubuntu20.04", "ubuntu22.04", "ubuntu22.10", "ubuntu23.04", "ubuntu23.10", "ubuntu24.04"},
			Debian: []string{"debian9", "debian10", "debian11"},
		}
		r.Result = obj
	}

	httputils.SetSuccess(c, r)
}
