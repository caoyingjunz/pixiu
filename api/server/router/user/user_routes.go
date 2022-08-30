package user

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"

	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/api/server/middleware"
	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
	"github.com/caoyingjunz/gopixiu/pkg/util"
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
	response := httputils.NewResponse()
	jwtKey := []byte(pixiu.CoreV1.User().GetJWTKey())

	var user types.User
	if err := ctx.ShouldBindJSON(&user); err != nil {
		httputils.SetFailed(ctx, response, err)
		return
	}

	expectedUser, err := pixiu.CoreV1.User().GetUserByName(context.TODO(), user.Name)
	if err != nil {
		httputils.SetFailed(ctx, response, err)
		return
	}

	// Compare login user password is correctly
	if err := bcrypt.CompareHashAndPassword([]byte(expectedUser.Password), []byte(user.Password)); err != nil {
		httputils.SetFailed(ctx, response, err)
		return
	}

	// Generate jwt
	expireTime := time.Now().Add(20 * time.Minute)
	claims := &middleware.Claims{
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expireTime.Unix(),
		},
		Id:   expectedUser.Id,
		Name: expectedUser.Name,
		Role: expectedUser.Role,
	}

	token, err := middleware.GenerateJWT(claims, jwtKey)
	if err != nil {
		httputils.SetFailed(ctx, response, err)
	}
	// Set token to response result
	response.Result = map[string]string{
		"token": token,
	}

	// Set token to gin.Context
	ctx.Set("token", token)

	httputils.SetSuccess(ctx, response)
}

func (u *userRouter) logout(ctx *gin.Context) {

}
