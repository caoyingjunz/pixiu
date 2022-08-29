package user

import (
	"context"

	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
	"github.com/caoyingjunz/gopixiu/pkg/util"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func (u *userRouter) createUser(ctx *gin.Context) {
	response := httputils.NewResponse()
	var user types.User

	if err := ctx.ShouldBindJSON(&user); err != nil {
		httputils.SetFailed(ctx, response, err)
		return
	}

	cryptPass, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		httputils.SetFailed(ctx, response, err)
		return
	}

	user.Password = string(cryptPass)

	if err := pixiu.CoreV1.User().Create(context.TODO(), &user); err != nil {
		httputils.SetFailed(ctx, response, err)
		return
	}

}

func (u *userRouter) deleteUser(ctx *gin.Context) {

}

func (u *userRouter) updateUser(ctx *gin.Context) {

}

func (u *userRouter) getUser(ctx *gin.Context) {
	response := httputils.NewResponse()
	uid, err := util.ParseInt64(ctx.Param("id"))
	if err != nil {
		httputils.SetFailed(ctx, response, err)
		return
	}
	response.Result, err = pixiu.CoreV1.User().Get(context.TODO(), uid)
	if err != nil {
		httputils.SetFailed(ctx, response, err)
	}
	httputils.SetSuccess(ctx, response)
}

func (u *userRouter) getAllUsers(ctx *gin.Context) {
	var err error
	response := httputils.NewResponse()
	response.Result, err = pixiu.CoreV1.User().GetAll(context.TODO())
	if err != nil {
		httputils.SetFailed(ctx, response, err)
	}
	httputils.SetSuccess(ctx, response)
}

func (u *userRouter) login(ctx *gin.Context) {

}

func (u *userRouter) logout(ctx *gin.Context) {

}
