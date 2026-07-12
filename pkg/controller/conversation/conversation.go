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

package conversation

import (
	"context"
	"fmt"
	"net/http"

	"k8s.io/klog/v2"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type Interface interface {
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (*types.Conversation, error)
	List(ctx context.Context, listOption types.ListOptions) (interface{}, error)
}

type controller struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &controller{cc: cfg, factory: f}
}

func (c *controller) Delete(ctx context.Context, id int64) error {
	old, err := c.factory.Assistant().Conversation().Get(ctx, id)
	if err != nil {
		klog.Errorf("failed to get conversation(%d): %v", id, err)
		return apierrors.ErrServerInternal
	}
	if old == nil {
		return apierrors.NewError(fmt.Errorf("conversation not found"), http.StatusNotFound)
	}

	if err = c.factory.Assistant().Conversation().Delete(ctx, id); err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return apierrors.NewError(fmt.Errorf("conversation not found"), http.StatusNotFound)
		}
		klog.Errorf("failed to delete conversation(%d): %v", id, err)
		return apierrors.ErrServerInternal
	}
	return nil
}

func (c *controller) Get(ctx context.Context, id int64) (*types.Conversation, error) {
	object, err := c.factory.Assistant().Conversation().Get(ctx, id)
	if err != nil {
		klog.Errorf("failed to get conversation(%d): %v", id, err)
		return nil, apierrors.ErrServerInternal
	}
	if object == nil {
		return nil, apierrors.NewError(fmt.Errorf("conversation not found"), http.StatusNotFound)
	}
	return modelToType(object), nil
}

func (c *controller) List(ctx context.Context, listOption types.ListOptions) (interface{}, error) {
	listOption.SetDefaultPageOption()

	pageResult := types.PageResult{
		PageRequest: types.PageRequest{
			Page:  listOption.Page,
			Limit: listOption.Limit,
		},
	}

	opts := []db.Options{
		db.WithProvider(listOption.Provider),
	}

	var err error
	pageResult.Total, err = c.factory.Assistant().Conversation().Count(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to count conversations: %v", err)
		return nil, apierrors.ErrServerInternal
	}

	offset := (listOption.Page - 1) * listOption.Limit
	opts = append(opts,
		db.WithModifyOrderByDesc(),
		db.WithOffset(offset),
		db.WithLimit(listOption.Limit),
	)

	objects, err := c.factory.Assistant().Conversation().List(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to list conversations: %v", err)
		return nil, apierrors.ErrServerInternal
	}

	items := make([]types.Conversation, 0)
	for i := range objects {
		items = append(items, *modelToType(&objects[i]))
	}
	pageResult.Items = items

	return pageResult, nil
}

func modelToType(object *model.Conversation) *types.Conversation {
	return &types.Conversation{
		PixiuMeta: types.PixiuMeta{
			Id:              object.Id,
			ResourceVersion: object.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   object.GmtCreate,
			GmtModified: object.GmtModified,
		},
		ProviderId:         object.ProviderId,
		Provider:           object.Provider,
		Model:              object.ModelName,
		Title:              object.Title,
		PreviousResponseId: object.PreviousResponseId,
		History:            object.History,
	}
}
