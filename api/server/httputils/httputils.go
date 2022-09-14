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
	"reflect"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
)

type any = interface{}

type Response struct {
	Code    int         `json:"code"` // 返回值
	Result  interface{} `json:"result,omitempty"`
	Message string      `json:"message,omitempty"`
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
	jwt.StandardClaims

	Id   int64  `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// GenerateToken 生成 token
func GenerateToken(uid int64, name string, jwtKey []byte) (string, error) {
	// Generate jwt, 临时有效期 360 分钟
	expireTime := time.Now().Add(360 * time.Minute)
	claims := &Claims{
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expireTime.Unix(),
		},
		Id:   uid,
		Name: name,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

func ParseToken(token string, jwtKey []byte) (*Claims, error) {
	var claims Claims
	t, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})
	if err != nil || !t.Valid {
		return nil, err
	}

	return &claims, nil
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

func ShouldBindAny(c *gin.Context, Obj ...any) error {
	for _, obj := range Obj {
		typeOfObj := reflect.TypeOf(obj)
		field := typeOfObj.Field(0)
		tag := strings.Split(string(field.Tag), ":")
		switch tag[0] {
		case "uri":
			if err := c.ShouldBindUri(&obj); err != nil {
				return err
			}
		case "json":
			if err := c.ShouldBindJSON(&obj); err != nil {
				return err
			}
		case "xml":
			if err := c.ShouldBindXML(&obj); err != nil {
				return err
			}
		case "query":
			if err := c.ShouldBindQuery(&obj); err != nil {
				return err
			}
		case "yaml":
			if err := c.ShouldBindYAML(&obj); err != nil {
				return err
			}
		case "toml":
			if err := c.ShouldBindTOML(&obj); err != nil {
				return err
			}
		case "header":
			if err := c.ShouldBindHeader(&obj); err != nil {
				return err
			}
		default:
			return fmt.Errorf("cannot resolve this type")
		}
	}
	return nil
}
