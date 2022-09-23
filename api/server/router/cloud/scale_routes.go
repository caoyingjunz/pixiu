package cloud

import (
	// "context"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/api/types"
	//"github.com/caoyingjunz/gopixiu/pkg/pixiu"
)

func (s *cloudRouter) scaleMod(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err          error
		scaleOption  types.ScaleOption
		updateOption types.UpdateOption
	)
	if err = c.ShouldBindUri(&scaleOption); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = c.ShouldBindJSON(&updateOption); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	//TODO
	//err = pixiu.CoreV1.Cloud().Scales(scaleOption.CloudName).Update(context.TODO(),updateOption,scaleOption)
	//if err != nil {
	//	httputils.SetFailed(c, r, err)
	//	return
	//}

	httputils.SetSuccess(c, r)
}
