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

package kubernetes

import (
	"context"
	"fmt"

	"github.com/caoyingjunz/gopixiu/pkg/db"
	"github.com/caoyingjunz/gopixiu/pkg/util/cipher"
	"github.com/caoyingjunz/gopixiu/pkg/util/intstr"
)

// ParseKubeConfigData 获取 kube config 解密之后的内容
func ParseKubeConfigData(ctx context.Context, factory db.ShareDaoFactory, cloudIntStr intstr.IntOrString) ([]byte, error) {
	var (
		cloudId int64
	)

	switch cloudIntStr.Type {
	case intstr.Int64:
		cloudId = cloudIntStr.Int64()
	case intstr.String:
		cloudObj, err := factory.Cloud().GetByName(ctx, cloudIntStr.String())
		if err != nil {
			return nil, fmt.Errorf("failed to get cloud: %v", err)
		}
		cloudId = cloudObj.Id
	default:
		return nil, fmt.Errorf("failed to get cloud: %s", cloudIntStr.String())
	}

	kubeConfigData, err := factory.KubeConfig().GetByCloud(ctx, cloudId)
	if err != nil {
		return nil, fmt.Errorf("failed to get %d cloud kubeConfig data: %v", cloudId, err)
	}

	return cipher.Decrypt(kubeConfigData.Config)
}
