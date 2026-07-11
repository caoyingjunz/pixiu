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

package assistant

import (
	"context"
	"net/http"
	"time"

	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/controller/conversation"
	"github.com/caoyingjunz/pixiu/pkg/controller/message"
	"github.com/caoyingjunz/pixiu/pkg/controller/provider"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type Getter interface {
	Assistant() Interface
}

type Interface interface {
	Stream(ctx context.Context, req *types.AIRespondRequest, emit func(*types.AIStreamEvent) error) (*types.AIRespondResponse, error)

	Provider() provider.Interface
	Conversation() conversation.Interface
	Message() message.Interface
}

type controller struct {
	cc           config.Config
	factory      db.ShareDaoFactory
	client       *http.Client
	provider     provider.Interface
	conversation conversation.Interface
	message      message.Interface
}

func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &controller{
		cc:           cfg,
		factory:      f,
		client:       &http.Client{Timeout: 60 * time.Second},
		provider:     provider.New(cfg, f),
		conversation: conversation.New(cfg, f),
		message:      message.New(cfg, f),
	}
}

func (c *controller) Provider() provider.Interface {
	return c.provider
}

func (c *controller) Conversation() conversation.Interface {
	return c.conversation
}

func (c *controller) Message() message.Interface {
	return c.message
}
