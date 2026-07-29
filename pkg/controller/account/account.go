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

package account

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"k8s.io/klog/v2"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type Interface interface {
	Create(ctx context.Context, req *types.CreateAIAccountRequest) error
	Update(ctx context.Context, req *types.UpdateAIAccountRequest) error
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (*types.AIAccount, error)
	List(ctx context.Context, listOption types.ListOptions) (interface{}, error)
}

type controller struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &controller{cc: cfg, factory: f}
}

func (c *controller) Create(ctx context.Context, req *types.CreateAIAccountRequest) error {
	if err := validateAccountFields(req.Name, req.APIKey, req.Model, req.ProviderId, true); err != nil {
		return err
	}
	if err := c.ensureProvider(ctx, req.ProviderId); err != nil {
		return err
	}
	userId, err := httputils.GetUserIdFromContext(ctx)
	if err != nil {
		return apierrors.NewError(fmt.Errorf("failed to get current user"), http.StatusUnauthorized)
	}

	_, err = c.factory.Assistant().Account().Create(ctx, &model.AIAccount{
		UserId:     userId,
		Name:       strings.TrimSpace(req.Name),
		APIKey:     strings.TrimSpace(req.APIKey),
		ModelName:  strings.TrimSpace(req.Model),
		ProviderId: req.ProviderId,
	})
	if err != nil {
		klog.Errorf("failed to create ai account: %v", err)
		return apierrors.ErrServerInternal
	}
	return nil
}

func (c *controller) Update(ctx context.Context, req *types.UpdateAIAccountRequest) error {
	if err := validateAccountFields(req.Name, req.APIKey, req.Model, req.ProviderId, false); err != nil {
		return err
	}
	if err := c.ensureProvider(ctx, req.ProviderId); err != nil {
		return err
	}

	old, err := c.factory.Assistant().Account().Get(ctx, req.Id)
	if err != nil {
		klog.Errorf("failed to get ai account(%d): %v", req.Id, err)
		return apierrors.ErrServerInternal
	}
	if old == nil {
		return apierrors.NewError(fmt.Errorf("ai account not found"), http.StatusNotFound)
	}
	if err = ensureAccountOwner(ctx, old); err != nil {
		return err
	}

	updates := map[string]interface{}{
		"name":        strings.TrimSpace(req.Name),
		"model":       strings.TrimSpace(req.Model),
		"provider_id": req.ProviderId,
	}
	if strings.TrimSpace(req.APIKey) != "" {
		updates["api_key"] = strings.TrimSpace(req.APIKey)
	}
	if err = c.factory.Assistant().Account().Update(ctx, req.Id, req.ResourceVersion, updates); err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return apierrors.NewError(fmt.Errorf("ai account not found or resource version conflict"), http.StatusConflict)
		}
		klog.Errorf("failed to update ai account(%d): %v", req.Id, err)
		return apierrors.ErrServerInternal
	}
	return nil
}

func (c *controller) Delete(ctx context.Context, id int64) error {
	object, err := c.factory.Assistant().Account().Get(ctx, id)
	if err != nil {
		klog.Errorf("failed to get ai account(%d): %v", id, err)
		return apierrors.ErrServerInternal
	}
	if object == nil {
		return apierrors.NewError(fmt.Errorf("ai account not found"), http.StatusNotFound)
	}
	if err = ensureAccountOwner(ctx, object); err != nil {
		return err
	}
	if err := c.factory.Assistant().Account().Delete(ctx, id); err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return apierrors.NewError(fmt.Errorf("ai account not found"), http.StatusNotFound)
		}
		klog.Errorf("failed to delete ai account(%d): %v", id, err)
		return apierrors.ErrServerInternal
	}
	return nil
}

func (c *controller) Get(ctx context.Context, id int64) (*types.AIAccount, error) {
	object, err := c.factory.Assistant().Account().Get(ctx, id)
	if err != nil {
		klog.Errorf("failed to get ai account(%d): %v", id, err)
		return nil, apierrors.ErrServerInternal
	}
	if object == nil {
		return nil, apierrors.NewError(fmt.Errorf("ai account not found"), http.StatusNotFound)
	}
	if err = ensureAccountOwner(ctx, object); err != nil {
		return nil, err
	}
	return modelToType(object), nil
}

func (c *controller) List(ctx context.Context, listOption types.ListOptions) (interface{}, error) {
	listOption.SetDefaultPageOption()
	userId, err := httputils.GetUserIdFromContext(ctx)
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("failed to get current user"), http.StatusUnauthorized)
	}
	opts := []db.Options{
		db.WithUser(userId),
		db.WithAIProviderId(listOption.ProviderId),
		db.WithNameLike(listOption.NameSelector),
	}

	total, err := c.factory.Assistant().Account().Count(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to count ai accounts: %v", err)
		return nil, apierrors.ErrServerInternal
	}
	offset := (listOption.Page - 1) * listOption.Limit
	opts = append(opts, db.WithModifyOrderByDesc(), db.WithOffset(offset), db.WithLimit(listOption.Limit))
	objects, err := c.factory.Assistant().Account().List(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to list ai accounts: %v", err)
		return nil, apierrors.ErrServerInternal
	}

	items := make([]types.AIAccount, 0, len(objects))
	for i := range objects {
		items = append(items, *modelToType(&objects[i]))
	}
	return types.PageResult{
		PageRequest: types.PageRequest{Page: listOption.Page, Limit: listOption.Limit},
		Total:       total,
		Items:       items,
	}, nil
}

func (c *controller) ensureProvider(ctx context.Context, providerId int64) error {
	provider, err := c.factory.Assistant().Provider().Get(ctx, providerId)
	if err != nil {
		klog.Errorf("failed to get ai provider(%d): %v", providerId, err)
		return apierrors.ErrServerInternal
	}
	if provider == nil {
		return apierrors.NewError(fmt.Errorf("ai provider not found"), http.StatusBadRequest)
	}
	return nil
}

func modelToType(object *model.AIAccount) *types.AIAccount {
	result := &types.AIAccount{
		PixiuMeta:  types.PixiuMeta{Id: object.Id, ResourceVersion: object.ResourceVersion},
		TimeMeta:   types.TimeMeta{GmtCreate: object.GmtCreate, GmtModified: object.GmtModified},
		Name:       object.Name,
		APIKey:     maskAPIKey(object.APIKey),
		Model:      object.ModelName,
		ProviderId: object.ProviderId,
		UserId:     object.UserId,
	}
	if object.Provider != nil {
		result.Provider = &types.AIProvider{
			PixiuMeta:   types.PixiuMeta{Id: object.Provider.Id, ResourceVersion: object.Provider.ResourceVersion},
			TimeMeta:    types.TimeMeta{GmtCreate: object.Provider.GmtCreate, GmtModified: object.Provider.GmtModified},
			Name:        object.Provider.Name,
			BaseURL:     object.Provider.BaseURL,
			Protocol:    string(object.Provider.Protocol),
			Description: object.Provider.Description,
			MaxTokens:   object.Provider.MaxTokens,
		}
	}
	return result
}

func ensureAccountOwner(ctx context.Context, object *model.AIAccount) error {
	userId, err := httputils.GetUserIdFromContext(ctx)
	if err != nil {
		return apierrors.NewError(fmt.Errorf("failed to get current user"), http.StatusUnauthorized)
	}
	if object == nil || object.UserId != userId {
		return apierrors.NewError(fmt.Errorf("ai account not found"), http.StatusNotFound)
	}
	return nil
}

func maskAPIKey(apiKey string) string {
	apiKey = strings.TrimSpace(apiKey)
	if len(apiKey) <= 8 {
		return "********"
	}
	return apiKey[:4] + "****" + apiKey[len(apiKey)-4:]
}

func validateAccountFields(name, apiKey, model string, providerId int64, requireAPIKey bool) error {
	if strings.TrimSpace(name) == "" {
		return apierrors.NewError(fmt.Errorf("name is required"), http.StatusBadRequest)
	}
	if requireAPIKey && strings.TrimSpace(apiKey) == "" {
		return apierrors.NewError(fmt.Errorf("api_key is required"), http.StatusBadRequest)
	}
	if strings.TrimSpace(model) == "" {
		return apierrors.NewError(fmt.Errorf("model is required"), http.StatusBadRequest)
	}
	if providerId <= 0 {
		return apierrors.NewError(fmt.Errorf("provider_id is required"), http.StatusBadRequest)
	}
	return nil
}
