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

package httputils

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
)

type Response struct {
	Code    int         `json:"code"`              // 返回的状态码
	Result  interface{} `json:"result,omitempty"`  // 正常返回时的数据，可以为任意数据结构
	Message string      `json:"message,omitempty"` // 异常返回时的错误信息
}

func (r *Response) Error() string {
	return r.Message
}

func (r *Response) SetCode(c int) {
	r.Code = c
}

func (r *Response) SetMessage(m interface{}) {
	switch msg := m.(type) {
	case error:
		r.Message = msg.Error()
	case string:
		r.Message = msg
	}
}

// SetSuccess 设置成功返回值
func SetSuccess(c *gin.Context, r *Response) {
	r.SetCode(http.StatusOK)
	c.JSON(http.StatusOK, r)
}

// SetFailed 设置错误返回值
func SetFailed(c *gin.Context, r *Response, err error) {
	r.SetMessage(err)
	c.JSON(http.StatusBadRequest, r)
}

func NewResponse() *Response {
	return &Response{
		Code: http.StatusBadRequest,
	}
}

type Claims struct {
	jwt.RegisteredClaims

	Id   int64  `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// GenerateToken 生成 token
func GenerateToken(uid int64, name string, jwtKey []byte) (string, error) {
	// Generate jwt, 临时有效期 360 分钟
	nowTime := time.Now()
	expiresTime := nowTime.Add(360 * time.Minute)
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresTime), // 过期时间
			IssuedAt:  jwt.NewNumericDate(nowTime),     // 签发时间
			NotBefore: jwt.NewNumericDate(nowTime),     // 生效时间
		},
		Id:   uid,
		Name: name,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

func ParseToken(tokenStr string, jwtKey []byte) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("failed to parse invailed token")
}

// ReadFile 从请求中获取指定文件内容
func ReadFile(c *gin.Context, f string) ([]byte, error) {
	fileHeader, err := c.FormFile(f)
	if err != nil {
		return nil, err
	}
	file, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return ioutil.ReadAll(file)
}
