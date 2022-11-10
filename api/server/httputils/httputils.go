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
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code    int         `json:"code"`              // 返回的状态码
	Result  interface{} `json:"result,omitempty"`  // 正常返回时的数据，可以为任意数据结构
	Message string      `json:"message,omitempty"` // 异常返回时的错误信息
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

func (r *Response) SetMessageWithCode(m interface{}, c int) {
	r.SetCode(c)
	r.SetMessage(m)
}

func (r *Response) Error() string {
	return r.Message
}

func (r *Response) String() string {
	return ""
}

// NewResponse 构造 http 返回值
// SetSuccess 时会自动设置 code 为 200
// SetFailed 时不需要设置状态码，SetCode 自定义状态码
func NewResponse() *Response {
	return &Response{}
}

// SetSuccess 设置成功返回值
func SetSuccess(c *gin.Context, r *Response) {
	r.SetCode(http.StatusOK)
	c.JSON(http.StatusOK, r)
}

// SetFailed 设置错误返回值
func SetFailed(c *gin.Context, r *Response, err error) {
	r.SetMessageWithCode(err, http.StatusBadRequest)
	c.JSON(http.StatusOK, r)
}

// SetFailedWithCode 设置错误返回值
func SetFailedWithCode(c *gin.Context, r *Response, code int, err error) {
	r.SetMessageWithCode(err, code)
	c.JSON(http.StatusOK, r)
}

// SetFailedWithAbortCode 设置错误，code 返回值并终止请求
func SetFailedWithAbortCode(c *gin.Context, r *Response, code int, err error) {
	r.SetMessageWithCode(err, code)
	c.JSON(http.StatusOK, r)
	c.Abort()
}
