/*
Copyright 2021 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package middleware

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"

	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/pkg/log"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
)

type Claims struct {
	jwt.StandardClaims

	Id   int64  `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

func InitMiddlewares(ginEngine *gin.Engine) {
	ginEngine.Use(LoggerToFile(), AuthN)
}

func LoggerToFile() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		// 处理请求操作
		c.Next()

		endTime := time.Now()

		latencyTime := endTime.Sub(startTime)

		reqMethod := c.Request.Method
		reqUri := c.Request.RequestURI
		statusCode := c.Writer.Status()
		clientIp := c.ClientIP()

		log.AccessLog.Infof("| %3d | %13v | %15s | %s | %s |", statusCode, latencyTime, clientIp, reqMethod, reqUri)
	}
}

func AuthN(c *gin.Context) {
	// Authentication 身份认证
	if c.Request.URL.Path == "/users/login" {
		return
	}

	r := httputils.NewResponse()
	token := c.GetHeader("Authorization")
	if len(token) == 0 {
		c.Abort()
		r.SetCode(http.StatusUnauthorized)
		httputils.SetFailed(c, r, errors.New("authorization header is not provided"))
		return
	}

	fields := strings.Fields(token)
	if len(fields) != 2 {
		c.Abort()
		r.SetCode(http.StatusUnauthorized)
		httputils.SetFailed(c, r, errors.New("invalid authorization header format"))
		return
	}

	if fields[0] != "Bearer" {
		c.Abort()
		r.SetCode(http.StatusUnauthorized)
		httputils.SetFailed(c, r, errors.New("unsupported authorization type"))
		return
	}

	accessToken := fields[1]
	jwtKey := pixiu.CoreV1.User().GetJWTKey()
	if _, err := ValidateJWT(accessToken, []byte(jwtKey)); err != nil {
		c.Abort()
		r.SetCode(http.StatusUnauthorized)
		httputils.SetFailed(c, r, err)
		return
	}

	c.Next()
}

func GenerateJWT(claims jwt.Claims, jwtKey []byte) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		return tokenString, err
	}

	return tokenString, nil
}

func ValidateJWT(token string, jwtKey []byte) (*Claims, error) {
	var claims Claims
	t, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})
	if err != nil || !t.Valid {
		return nil, err
	}

	return &claims, nil
}
