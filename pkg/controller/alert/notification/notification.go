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

package notification

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
	Delete(ctx context.Context, notificationId int64) error
	List(ctx context.Context, listOption types.ListOptions) (interface{}, error)
}

type controller struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &controller{cc: cfg, factory: f}
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
		db.WithAlertRuleId(listOption.RuleId),
		db.WithAlertEventId(listOption.EventId),
		db.WithTitleLike(listOption.NameSelector),
	}

	var err error
	pageResult.Total, err = c.factory.Alert().Notification().Count(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to count alert notifications: %v", err)
		pageResult.Message = err.Error()
	}

	offset := (listOption.Page - 1) * listOption.Limit
	opts = append(opts, []db.Options{
		db.WithModifyOrderByDesc(),
		db.WithOffset(offset),
		db.WithLimit(listOption.Limit),
	}...)

	objects, err := c.factory.Alert().Notification().List(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to list alert notifications: %v", err)
		pageResult.Message = err.Error()
		return nil, apierrors.ErrServerInternal
	}

	items := make([]types.AlertNotification, 0, len(objects))
	for i := range objects {
		items = append(items, *modelToType(&objects[i]))
	}
	pageResult.Items = items

	return pageResult, nil
}

func (c *controller) Delete(ctx context.Context, notificationId int64) error {
	if err := c.factory.Alert().Notification().Delete(ctx, notificationId); err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return apierrors.NewError(fmt.Errorf("alert notification not found"), http.StatusNotFound)
		}
		klog.Errorf("failed to delete alert notification(%d): %v", notificationId, err)
		return apierrors.ErrServerInternal
	}
	return nil
}

func modelToType(object *model.AlertNotification) *types.AlertNotification {
	return &types.AlertNotification{
		PixiuMeta: types.PixiuMeta{Id: object.Id, ResourceVersion: object.ResourceVersion},
		TimeMeta:  types.TimeMeta{GmtCreate: object.GmtCreate, GmtModified: object.GmtModified},
		EventId:   object.EventId, RuleId: object.RuleId, Channel: object.Channel,
		Title: object.Title, Content: object.Content, Status: object.Status,
		RetryCount: object.RetryCount, ErrorMsg: object.ErrorMsg,
		Severity: object.Severity, Labels: object.Labels, ChannelName: object.ChannelName,
	}
}
