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

package iface

import (
	"context"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/db/tx_func"
)

type ClusterInterface interface {
	Create(ctx context.Context, object *model.Cluster, fns ...tx_func.TxFunc) (*model.Cluster, error)
	Update(ctx context.Context, cid int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, cid int64) (*model.Cluster, error)
	Get(ctx context.Context, cid int64) (*model.Cluster, error)
	List(ctx context.Context) ([]model.Cluster, error)

	GetClusterByName(ctx context.Context, name string) (*model.Cluster, error)
}
