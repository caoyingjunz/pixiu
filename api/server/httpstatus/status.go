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

package httpstatus

import "errors"

var (
	ParamsError       = errors.New("参数错误")
	OperateFailed     = errors.New("操作失败")
	NoPermission      = errors.New("无权限")
	InnerError        = errors.New("inner error")
	NoUserIdError     = errors.New("请登录")
	RoleExistError    = errors.New("角色已存在")
	RoleNotExistError = errors.New("角色不存在")
	MenusExistError   = errors.New("权限已存在")
)
