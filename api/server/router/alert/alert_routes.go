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
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type ruleMeta struct {
	RuleId int64 `uri:"ruleId"`
}

type eventMeta struct {
	EventId int64 `uri:"eventId"`
}

type silenceMeta struct {
	SilenceId int64 `uri:"silenceId"`
}

type channelMeta struct {
	ChannelId int64 `uri:"channelId"`
}

type notifyMeta struct {
	NotificationId int64 `uri:"notificationId"`
}

func (r *router) createRule(c *gin.Context) {
	resp := httputils.NewResponse()
	var req types.CreateAlertRuleRequest
	if err := httputils.ShouldBindAny(c, &req, nil, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Alert().Rule().Create(c, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) updateRule(c *gin.Context) {
	resp := httputils.NewResponse()
	var (
		meta ruleMeta
		req  types.UpdateAlertRuleRequest
	)
	if err := httputils.ShouldBindAny(c, &req, &meta, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Alert().Rule().Update(c, meta.RuleId, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) deleteRule(c *gin.Context) {
	resp := httputils.NewResponse()
	var meta ruleMeta
	if err := httputils.ShouldBindAny(c, nil, &meta, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Alert().Rule().Delete(c, meta.RuleId); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) getRule(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		meta ruleMeta
		err  error
	)
	if err = httputils.ShouldBindAny(c, nil, &meta, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if resp.Result, err = r.c.Alert().Rule().Get(c, meta.RuleId); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) listRules(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		listOption types.ListOptions
		err        error
	)
	if err = httputils.ShouldBindAny(c, nil, nil, &listOption); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if resp.Result, err = r.c.Alert().Rule().List(c, listOption); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) getEvent(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		meta eventMeta
		err  error
	)
	if err = httputils.ShouldBindAny(c, nil, &meta, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if resp.Result, err = r.c.Alert().Event().Get(c, meta.EventId); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) listEvents(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		listOption types.ListOptions
		err        error
	)
	if err = httputils.ShouldBindAny(c, nil, nil, &listOption); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if resp.Result, err = r.c.Alert().Event().List(c, listOption); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) updateEventStatus(c *gin.Context) {
	resp := httputils.NewResponse()
	var (
		meta eventMeta
		req  types.UpdateAlertEventStatusRequest
	)
	if err := httputils.ShouldBindAny(c, &req, &meta, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Alert().Event().UpdateStatus(c, meta.EventId, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) createChannel(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		req types.CreateAlertChannelRequest
		err error
	)
	if err = httputils.ShouldBindAny(c, &req, nil, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err = r.c.Alert().Channel().Create(c, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) updateChannel(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		meta channelMeta
		req  types.UpdateAlertChannelRequest
		err  error
	)
	if err = httputils.ShouldBindAny(c, &req, &meta, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err = r.c.Alert().Channel().Update(c, meta.ChannelId, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) deleteChannel(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		meta channelMeta
		err  error
	)
	if err = httputils.ShouldBindAny(c, nil, &meta, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err = r.c.Alert().Channel().Delete(c, meta.ChannelId); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) getChannel(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		meta channelMeta
		err  error
	)
	if err = httputils.ShouldBindAny(c, nil, &meta, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if resp.Result, err = r.c.Alert().Channel().Get(c, meta.ChannelId); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) listChannels(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		listOption types.ListOptions
		err        error
	)
	if err = httputils.ShouldBindAny(c, nil, nil, &listOption); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if resp.Result, err = r.c.Alert().Channel().List(c, listOption); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) pingChannel(c *gin.Context) {
	resp := httputils.NewResponse()

	var req types.PingAlertChannelRequest
	if err := httputils.ShouldBindAny(c, &req, nil, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Alert().Channel().Ping(c, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) listNotifications(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		listOption types.ListOptions
		err        error
	)
	if err = httputils.ShouldBindAny(c, nil, nil, &listOption); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if resp.Result, err = r.c.Alert().Notification().List(c, listOption); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) deleteNotification(c *gin.Context) {
	resp := httputils.NewResponse()
	var (
		meta notifyMeta
		err  error
	)
	if err = httputils.ShouldBindAny(c, nil, &meta, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err = r.c.Alert().Notification().Delete(c, meta.NotificationId); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) createSilence(c *gin.Context) {
	resp := httputils.NewResponse()
	var req types.CreateAlertSilenceRequest
	if err := httputils.ShouldBindAny(c, &req, nil, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Alert().Silence().Create(c, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) updateSilence(c *gin.Context) {
	resp := httputils.NewResponse()
	var (
		meta silenceMeta
		req  types.UpdateAlertSilenceRequest
	)
	if err := httputils.ShouldBindAny(c, &req, &meta, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Alert().Silence().Update(c, meta.SilenceId, &req); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) deleteSilence(c *gin.Context) {
	resp := httputils.NewResponse()
	var meta silenceMeta
	if err := httputils.ShouldBindAny(c, nil, &meta, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if err := r.c.Alert().Silence().Delete(c, meta.SilenceId); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) getSilence(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		meta silenceMeta
		err  error
	)
	if err = httputils.ShouldBindAny(c, nil, &meta, nil); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if resp.Result, err = r.c.Alert().Silence().Get(c, meta.SilenceId); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}

func (r *router) listSilences(c *gin.Context) {
	resp := httputils.NewResponse()

	var (
		listOption types.ListOptions
		err        error
	)
	if err = httputils.ShouldBindAny(c, nil, nil, &listOption); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	if resp.Result, err = r.c.Alert().Silence().List(c, listOption); err != nil {
		httputils.SetFailed(c, resp, err)
		return
	}
	httputils.SetSuccess(c, resp)
}
