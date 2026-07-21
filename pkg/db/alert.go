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

package db

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type AlertInterface interface {
	Rule() AlertRuleInterface
	Event() AlertEventInterface
	Notification() AlertNotificationInterface
	Channel() AlertChannelInterface
	Silence() AlertSilenceInterface
}

type alert struct {
	db *gorm.DB
}

func newAlert(db *gorm.DB) AlertInterface {
	return &alert{db: db}
}

func (a *alert) Rule() AlertRuleInterface   { return &alertRule{db: a.db} }
func (a *alert) Event() AlertEventInterface { return &alertEvent{db: a.db} }
func (a *alert) Notification() AlertNotificationInterface {
	return &alertNotification{db: a.db}
}
func (a *alert) Channel() AlertChannelInterface { return &alertChannel{db: a.db} }
func (a *alert) Silence() AlertSilenceInterface { return &alertSilence{db: a.db} }

type AlertRuleInterface interface {
	Create(ctx context.Context, object *model.AlertRule) (*model.AlertRule, error)
	Update(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (*model.AlertRule, error)
	List(ctx context.Context, opts ...Options) ([]model.AlertRule, error)
	Count(ctx context.Context, opts ...Options) (int64, error)
}

type alertRule struct{ db *gorm.DB }

func (a *alertRule) Create(ctx context.Context, object *model.AlertRule) (*model.AlertRule, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now
	if err := a.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (a *alertRule) Update(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1
	f := a.db.WithContext(ctx).Model(&model.AlertRule{}).Where("id = ? and resource_version = ?", id, resourceVersion).Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}
	return nil
}

func (a *alertRule) Delete(ctx context.Context, id int64) error {
	f := a.db.WithContext(ctx).Where("id = ?", id).Delete(&model.AlertRule{})
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}
	return nil
}

func (a *alertRule) Get(ctx context.Context, id int64) (*model.AlertRule, error) {
	var object model.AlertRule
	if err := a.db.WithContext(ctx).Where("id = ?", id).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (a *alertRule) List(ctx context.Context, opts ...Options) ([]model.AlertRule, error) {
	var objects []model.AlertRule
	tx := a.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Find(&objects).Error; err != nil {
		return nil, err
	}
	return objects, nil
}

func (a *alertRule) Count(ctx context.Context, opts ...Options) (int64, error) {
	var total int64
	tx := a.db.WithContext(ctx).Model(&model.AlertRule{})
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

type AlertEventInterface interface {
	Create(ctx context.Context, object *model.AlertEvent) (*model.AlertEvent, error)
	Update(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error
	Get(ctx context.Context, id int64) (*model.AlertEvent, error)
	List(ctx context.Context, opts ...Options) ([]model.AlertEvent, error)
	Count(ctx context.Context, opts ...Options) (int64, error)
	GetActive(ctx context.Context, ruleId int64, resourceType, resourceName string) (*model.AlertEvent, error)
}

type alertEvent struct{ db *gorm.DB }

func (a *alertEvent) Create(ctx context.Context, object *model.AlertEvent) (*model.AlertEvent, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now
	if err := a.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (a *alertEvent) Update(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1
	f := a.db.WithContext(ctx).Model(&model.AlertEvent{}).Where("id = ? and resource_version = ?", id, resourceVersion).Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}
	return nil
}

func (a *alertEvent) Get(ctx context.Context, id int64) (*model.AlertEvent, error) {
	var object model.AlertEvent
	if err := a.db.WithContext(ctx).Where("id = ?", id).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (a *alertEvent) List(ctx context.Context, opts ...Options) ([]model.AlertEvent, error) {
	var objects []model.AlertEvent
	tx := a.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Find(&objects).Error; err != nil {
		return nil, err
	}
	return objects, nil
}

func (a *alertEvent) Count(ctx context.Context, opts ...Options) (int64, error) {
	var total int64
	tx := a.db.WithContext(ctx).Model(&model.AlertEvent{})
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (a *alertEvent) GetActive(ctx context.Context, ruleId int64, resourceType, resourceName string) (*model.AlertEvent, error) {
	var object model.AlertEvent
	err := a.db.WithContext(ctx).
		Where("rule_id = ? and resource_type = ? and resource_name = ? and status = ?",
			ruleId, resourceType, resourceName, model.AlertEventStatusFiring).
		Order("id desc").
		First(&object).Error
	if err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

type AlertNotificationInterface interface {
	Create(ctx context.Context, object *model.AlertNotification) (*model.AlertNotification, error)
	Update(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, opts ...Options) (int64, error)
	Get(ctx context.Context, id int64) (*model.AlertNotification, error)
	List(ctx context.Context, opts ...Options) ([]model.AlertNotification, error)

	Count(ctx context.Context, opts ...Options) (int64, error)
	ListPending(ctx context.Context, limit int) ([]model.AlertNotification, error)
}

type alertNotification struct{ db *gorm.DB }

func (a *alertNotification) Create(ctx context.Context, object *model.AlertNotification) (*model.AlertNotification, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now
	if err := a.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (a *alertNotification) Update(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1
	f := a.db.WithContext(ctx).Model(&model.AlertNotification{}).Where("id = ? and resource_version = ?", id, resourceVersion).Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}
	return nil
}

func (a *alertNotification) Get(ctx context.Context, id int64) (*model.AlertNotification, error) {
	var object model.AlertNotification
	if err := a.db.WithContext(ctx).Where("id = ?", id).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (a *alertNotification) List(ctx context.Context, opts ...Options) ([]model.AlertNotification, error) {
	var objects []model.AlertNotification
	tx := a.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Find(&objects).Error; err != nil {
		return nil, err
	}
	return objects, nil
}

func (a *alertNotification) Count(ctx context.Context, opts ...Options) (int64, error) {
	var total int64
	tx := a.db.WithContext(ctx).Model(&model.AlertNotification{})
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (a *alertNotification) ListPending(ctx context.Context, limit int) ([]model.AlertNotification, error) {
	if limit <= 0 {
		limit = 50
	}
	var objects []model.AlertNotification
	err := a.db.WithContext(ctx).
		Where("status = ?", model.AlertNotificationStatusPending).
		Order("id asc").
		Limit(limit).
		Find(&objects).Error
	if err != nil {
		return nil, err
	}
	return objects, nil
}

func (a *alertNotification) Delete(ctx context.Context, opts ...Options) (int64, error) {
	tx := a.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	f := tx.Delete(&model.AlertNotification{})
	if f.Error != nil {
		return 0, f.Error
	}
	return f.RowsAffected, nil
}

type AlertChannelInterface interface {
	Create(ctx context.Context, object *model.AlertChannel) (*model.AlertChannel, error)
	Update(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (*model.AlertChannel, error)
	List(ctx context.Context, opts ...Options) ([]model.AlertChannel, error)
	Count(ctx context.Context, opts ...Options) (int64, error)
}

type alertChannel struct{ db *gorm.DB }

func (a *alertChannel) Create(ctx context.Context, object *model.AlertChannel) (*model.AlertChannel, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now
	if err := a.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (a *alertChannel) Update(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1
	f := a.db.WithContext(ctx).Model(&model.AlertChannel{}).Where("id = ? and resource_version = ?", id, resourceVersion).Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}
	return nil
}

func (a *alertChannel) Delete(ctx context.Context, id int64) error {
	f := a.db.WithContext(ctx).Where("id = ?", id).Delete(&model.AlertChannel{})
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}
	return nil
}

func (a *alertChannel) Get(ctx context.Context, id int64) (*model.AlertChannel, error) {
	var object model.AlertChannel
	if err := a.db.WithContext(ctx).Where("id = ?", id).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (a *alertChannel) List(ctx context.Context, opts ...Options) ([]model.AlertChannel, error) {
	var objects []model.AlertChannel
	tx := a.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Find(&objects).Error; err != nil {
		return nil, err
	}
	return objects, nil
}

func (a *alertChannel) Count(ctx context.Context, opts ...Options) (int64, error) {
	var total int64
	tx := a.db.WithContext(ctx).Model(&model.AlertChannel{})
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

type AlertSilenceInterface interface {
	Create(ctx context.Context, object *model.AlertSilence) (*model.AlertSilence, error)
	Update(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (*model.AlertSilence, error)
	List(ctx context.Context, opts ...Options) ([]model.AlertSilence, error)
	Count(ctx context.Context, opts ...Options) (int64, error)
	ListActive(ctx context.Context, now time.Time) ([]model.AlertSilence, error)
}

type alertSilence struct{ db *gorm.DB }

func (a *alertSilence) Create(ctx context.Context, object *model.AlertSilence) (*model.AlertSilence, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now
	if err := a.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (a *alertSilence) Update(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1
	f := a.db.WithContext(ctx).Model(&model.AlertSilence{}).Where("id = ? and resource_version = ?", id, resourceVersion).Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}
	return nil
}

func (a *alertSilence) Delete(ctx context.Context, id int64) error {
	f := a.db.WithContext(ctx).Where("id = ?", id).Delete(&model.AlertSilence{})
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}
	return nil
}

func (a *alertSilence) Get(ctx context.Context, id int64) (*model.AlertSilence, error) {
	var object model.AlertSilence
	if err := a.db.WithContext(ctx).Where("id = ?", id).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (a *alertSilence) List(ctx context.Context, opts ...Options) ([]model.AlertSilence, error) {
	var objects []model.AlertSilence
	tx := a.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Find(&objects).Error; err != nil {
		return nil, err
	}
	return objects, nil
}

func (a *alertSilence) Count(ctx context.Context, opts ...Options) (int64, error) {
	var total int64
	tx := a.db.WithContext(ctx).Model(&model.AlertSilence{})
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (a *alertSilence) ListActive(ctx context.Context, now time.Time) ([]model.AlertSilence, error) {
	var objects []model.AlertSilence
	err := a.db.WithContext(ctx).
		Where("enabled = ? and starts_at <= ? and ends_at >= ?", true, now, now).
		Find(&objects).Error
	if err != nil {
		return nil, err
	}
	return objects, nil
}
