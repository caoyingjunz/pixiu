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

package core

import (
	"context"
	"time"

	"github.com/caoyingjunz/gopixiu/pkg/db"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	"github.com/caoyingjunz/gopixiu/pkg/log"
	"github.com/caoyingjunz/gopixiu/pkg/types"
)

type AuditGetter interface {
	Audit() AuditInterface
}

type AuditInterface interface {
	Create(ctx context.Context, event *types.Event) error
	List(ctx context.Context, duration string) ([]types.Event, error)

	Run(stopCh chan struct{})
}

type audit struct {
	factory db.ShareDaoFactory
}

func newAudit(c *pixiu) AuditInterface {
	return &audit{
		factory: c.factory,
	}
}

func (audit *audit) Create(ctx context.Context, event *types.Event) error {
	if _, err := audit.factory.Audit().Create(ctx, &model.Event{
		User:     event.User,
		ClientIP: event.ClientIP,
		Operator: string(event.Operator),
		Object:   string(event.Object),
		Message:  event.Message,
	}); err != nil {
		log.Logger.Errorf("failed to create event %s: %s: %v", event.User, event.ClientIP, err)
		return err
	}

	return nil
}

// List duration 以小时为单位
func (audit *audit) List(ctx context.Context, duration string) ([]types.Event, error) {
	now := time.Now()
	t, err := time.ParseDuration("-" + duration)
	if err != nil {
		log.Logger.Errorf("failed to parse % duration: %v", duration, err)
		return nil, err
	}
	now.Add(t)

	events, err := audit.factory.Audit().List(ctx, now)
	if err != nil {
		log.Logger.Errorf("failed to list recently %s events: %v", duration, err)
		return nil, err
	}
	var es []types.Event
	for _, e := range events {
		es = append(es, *audit.model2Type(&e))
	}

	return es, nil
}

func (audit *audit) model2Type(obj *model.Event) *types.Event {
	return &types.Event{
		User:     obj.User,
		ClientIP: obj.ClientIP,
		Operator: types.EventType(obj.Operator),
		Object:   types.ResourceType(obj.Object),
		Message:  obj.Message,
	}
}

// Run 启动定时清理
// 默认保留 7 天的审计事件
func (audit *audit) Run(stopCh chan struct{}) {
	// 每天清理一次
	go audit.run(time.Second*3600*24, stopCh)
}

func (audit *audit) run(duration time.Duration, stopCh chan struct{}) {
	log.Logger.Infof("starting audit clean job")

	for {
		select {
		case <-time.After(duration): // 每天清理一次
			now := time.Now()
			log.Logger.Infof("starting to clean audit events at %v", now)

			// 默认保留 7 天的审计事件
			cleanTime := now.AddDate(0, 0, -7)
			if err := audit.factory.Audit().Delete(context.TODO(), cleanTime); err != nil {
				log.Logger.Errorf("failed to delete audit events: %v", err)
			}
		case <-stopCh:
			return
		}
	}
}
