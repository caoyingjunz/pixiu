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

package alert

import (
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/controller/alert/channel"
	"github.com/caoyingjunz/pixiu/pkg/controller/alert/event"
	"github.com/caoyingjunz/pixiu/pkg/controller/alert/notification"
	"github.com/caoyingjunz/pixiu/pkg/controller/alert/rule"
	"github.com/caoyingjunz/pixiu/pkg/controller/alert/silence"
	"github.com/caoyingjunz/pixiu/pkg/db"
)

type Getter interface {
	Alert() Interface
}

type Interface interface {
	Rule() rule.Interface
	Event() event.Interface
	Channel() channel.Interface
	Notification() notification.Interface
	Silence() silence.Interface
}

type controller struct {
	cc           config.Config
	factory      db.ShareDaoFactory
	rule         rule.Interface
	event        event.Interface
	channel      channel.Interface
	notification notification.Interface
	silence      silence.Interface
}

func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &controller{
		cc:           cfg,
		factory:      f,
		rule:         rule.New(cfg, f),
		event:        event.New(cfg, f),
		channel:      channel.New(cfg, f),
		notification: notification.New(cfg, f),
		silence:      silence.New(cfg, f),
	}
}

func (c *controller) Rule() rule.Interface {
	return c.rule
}

func (c *controller) Event() event.Interface {
	return c.event
}

func (c *controller) Channel() channel.Interface {
	return c.channel
}

func (c *controller) Notification() notification.Interface {
	return c.notification
}

func (c *controller) Silence() silence.Interface {
	return c.silence
}
