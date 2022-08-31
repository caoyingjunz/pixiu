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

func (u *userRouter) create(c *gin.Context) {
	r := httputils.NewResponse()

	var user types.User
	if err := c.ShouldBindJSON(&user); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	cryptPass, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	user.Password = string(cryptPass)
	if err := pixiu.CoreV1.User().Create(context.TODO(), &user); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
}

func (u *userRouter) delete(c *gin.Context) {
	r := httputils.NewResponse()

	uid, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if err := pixiu.CoreV1.User().Delete(context.TODO(), uid); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (u *userRouter) update(c *gin.Context) {
	r := httputils.NewResponse()

	var err error
	var user types.User
	if err = c.ShouldBindJSON(&user); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	user.Id, err = util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if user.Password != "" {
		cryptPass, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			httputils.SetFailed(c, r, err)
			return
		}
		user.Password = string(cryptPass)
	}

	if err := pixiu.CoreV1.User().Update(context.TODO(), &user); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (u *userRouter) get(c *gin.Context) {
	r := httputils.NewResponse()

	uid, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	r.Result, err = pixiu.CoreV1.User().Get(context.TODO(), uid)
	if err != nil {
		httputils.SetFailed(c, r, err)
	}

	httputils.SetSuccess(c, r)
}

func (u *userRouter) list(c *gin.Context) {
	var err error
	r := httputils.NewResponse()
	r.Result, err = pixiu.CoreV1.User().List(context.TODO())
	if err != nil {
		httputils.SetFailed(c, r, err)
	}

	httputils.SetSuccess(c, r)
}

func (u *userRouter) login(c *gin.Context) {
	r := httputils.NewResponse()
	jwtKey := []byte(pixiu.CoreV1.User().GetJWTKey())

	var user types.User
	if err := c.ShouldBindJSON(&user); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	expectedUser, err := pixiu.CoreV1.User().GetByName(context.TODO(), user.Name)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	// Compare login user password is correctly
	if err := bcrypt.CompareHashAndPassword([]byte(expectedUser.Password), []byte(user.Password)); err != nil {
		httputils.SetFailed(c, r, err)
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
		httputils.SetFailed(c, r, err)
	}
	// Set token to r result
	r.Result = map[string]string{
		"token": token,
	}

	// Set token to gin.Context
	c.Set("token", token)

	httputils.SetSuccess(c, r)
}

func (u *userRouter) logout(c *gin.Context) {

}
