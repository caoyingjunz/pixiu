/*
Copyright 2024 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package cluster

import (
	"context"
	"fmt"

	"github.com/caoyingjunz/pixiu/pkg/types"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
	sshutil "github.com/caoyingjunz/pixiu/pkg/util/ssh"
)

// ResolveSSHConfigForHost 根据已注册节点的 IP 解析 SSH 连接参数（端口默认 22，账号与凭证来自节点 auth JSON）。
func (c *cluster) ResolveSSHConfigForHost(ctx context.Context, host string) (*types.WebSSHRequest, error) {
	n, err := c.factory.Plan().GetNodeByIP(ctx, host)
	if err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return nil, utilerrors.ErrNodeNotFound
		}
		return nil, err
	}

	var auth types.PlanNodeAuth
	if err = auth.Unmarshal(n.Auth); err != nil {
		return nil, fmt.Errorf("解析节点认证信息失败: %w", err)
	}

	req, err := sshutil.ResolveAuth(&auth)
	if err != nil {
		return nil, err
	}
	req.Host = n.Ip

	return req, nil
}
