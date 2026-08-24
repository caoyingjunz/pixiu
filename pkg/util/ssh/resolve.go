/*
Copyright 2026 The Pixiu Authors.

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

package ssh

import (
	"fmt"

	"github.com/caoyingjunz/pixiu/pkg/types"
)

// ResolveAuth 将节点认证配置解析为 SSH 连接参数（端口/账号/凭据；Host 由调用方填充）
func ResolveAuth(auth *types.PlanNodeAuth) (*types.WebSSHRequest, error) {
	if auth == nil {
		return nil, fmt.Errorf("认证配置为空")
	}
	req := &types.WebSSHRequest{Port: auth.SSHPort()}
	switch auth.Type {
	case types.PasswordAuth:
		if auth.Password == nil || auth.Password.Password == "" {
			return nil, fmt.Errorf("节点未配置 SSH 密码")
		}
		req.User = auth.Password.User
		if req.User == "" {
			req.User = "root"
		}
		req.Password = auth.Password.Password
	case types.KeyAuth:
		if auth.Key == nil || auth.Key.Data == "" {
			return nil, fmt.Errorf("节点未配置 SSH 私钥")
		}
		req.User = "root"
		req.PrivateKey = auth.Key.Data
	default:
		return nil, fmt.Errorf("不支持的认证类型: %s", auth.Type)
	}
	return req, nil
}
