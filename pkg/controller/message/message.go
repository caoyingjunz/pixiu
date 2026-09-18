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

package message

import (
	"context"
	"fmt"
	"net/http"

	"k8s.io/klog/v2"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
	"gorm.io/gorm"
)

type Interface interface {
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (*types.Message, error)
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
	old, err := c.factory.Assistant().Message().Get(ctx, id)
	if err != nil {
		klog.Errorf("failed to get message(%d): %v", id, err)
		return apierrors.ErrServerInternal
	}
	if old == nil || !c.owned(ctx, old.ConversationId) {
		return apierrors.NewError(fmt.Errorf("message not found"), http.StatusNotFound)
	}

	if err = c.factory.Assistant().Message().Delete(ctx, id); err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return apierrors.NewError(fmt.Errorf("message not found"), http.StatusNotFound)
		}
		klog.Errorf("failed to delete message(%d): %v", id, err)
		return apierrors.ErrServerInternal
	}
	return nil
}

func (c *controller) Get(ctx context.Context, id int64) (*types.Message, error) {
	object, err := c.factory.Assistant().Message().Get(ctx, id)
	if err != nil {
		klog.Errorf("failed to get message(%d): %v", id, err)
		return nil, apierrors.ErrServerInternal
	}
	if object == nil || !c.owned(ctx, object.ConversationId) {
		return nil, apierrors.NewError(fmt.Errorf("message not found"), http.StatusNotFound)
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

	userID, err := httputils.GetUserIdFromContext(ctx)
	if err != nil {
		return nil, err
	}
	opts := []db.Options{
		func(tx *gorm.DB) *gorm.DB {
			return tx.Where("conversation_id IN (SELECT id FROM conversations WHERE user_id = ?)", userID)
		},
		db.WithProvider(listOption.Provider),
		db.WithConversationId(listOption.ConversationId),
	}

	pageResult.Total, err = c.factory.Assistant().Message().Count(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to count messages: %v", err)
		return nil, apierrors.ErrServerInternal
	}

	offset := (listOption.Page - 1) * listOption.Limit
	opts = append(opts,
		db.WithModifyOrderByDesc(),
		db.WithOffset(offset),
		db.WithLimit(listOption.Limit),
	)

	objects, err := c.factory.Assistant().Message().List(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to list messages: %v", err)
		return nil, apierrors.ErrServerInternal
	}

	items := make([]types.Message, 0)
	for i := range objects {
		items = append(items, *modelToType(&objects[i]))
	}
	pageResult.Items = items

	return pageResult, nil
}

func modelToType(object *model.Message) *types.Message {
	return &types.Message{
		PixiuMeta: types.PixiuMeta{
			Id:              object.Id,
			ResourceVersion: object.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   object.GmtCreate,
			GmtModified: object.GmtModified,
		},
		RequestId:       object.RequestId,
		ProviderId:      object.ProviderId,
		ConversationId:  object.ConversationId,
		Provider:        object.Provider,
		Model:           object.ModelName,
		ResponseId:      object.ResponseId,
		InputText:       object.InputText,
		OutputText:      object.OutputText,
		Success:         object.Success,
		ErrorMessage:    object.ErrorMessage,
		Duration:        object.Duration,
		InputTokens:     object.InputTokens,
		OutputTokens:    object.OutputTokens,
		TotalTokens:     object.TotalTokens,
		CachedTokens:    object.CachedTokens,
		ReasoningTokens: object.ReasoningTokens,
	}
}

func (c *controller) owned(ctx context.Context, id int64) bool {
	userID, err := httputils.GetUserIdFromContext(ctx)
	if err != nil {
		return false
	}
	object, err := c.factory.Assistant().Conversation().Get(ctx, id)
	return err == nil && object != nil && object.UserId != nil && *object.UserId == userID
}
