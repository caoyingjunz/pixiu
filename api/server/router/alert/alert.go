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

	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

const (
	alertBaseURL        = "/pixiu/alerts"
	alertRuleBaseURL    = alertBaseURL + "/rules"
	alertEventBaseURL   = alertBaseURL + "/events"
	alertChannelBaseURL = alertBaseURL + "/channels"
	alertNotifyBaseURL  = alertBaseURL + "/notifications"
	alertSilenceBaseURL = alertBaseURL + "/silences"
)

type router struct {
	c controller.PixiuInterface
}

func NewRouter(o *options.Options) {
	r := &router{c: o.Controller}
	r.initRoutes(o.HttpEngine)
}

func (r *router) initRoutes(ginEngine *gin.Engine) {
	ruleGroup := &apiregistry.Group{
		Name:    "告警",
		BaseURL: alertRuleBaseURL,
		Entries: []apiregistry.RouteEntry{
			{Method: "POST", RelativePath: "", Handler: r.createRule, Description: "Create alert rule"},
			{Method: "PUT", RelativePath: "/:ruleId", Handler: r.updateRule, Description: "Update alert rule"},
			{Method: "DELETE", RelativePath: "/:ruleId", Handler: r.deleteRule, Description: "Delete alert rule"},
			{Method: "GET", RelativePath: "", Handler: r.listRules, Description: "List alert rules"},
			{Method: "GET", RelativePath: "/:ruleId", Handler: r.getRule, Description: "Get alert rule"},
		},
	}
	ruleGroup.Register(ginEngine.Group(alertRuleBaseURL), r.c.APIResource())

	eventGroup := &apiregistry.Group{
		Name:    "告警",
		BaseURL: alertEventBaseURL,
		Entries: []apiregistry.RouteEntry{
			{Method: "GET", RelativePath: "", Handler: r.listEvents, Description: "List alert events"},
			{Method: "GET", RelativePath: "/:eventId", Handler: r.getEvent, Description: "Get alert event"},
			{Method: "PUT", RelativePath: "/:eventId/status", Handler: r.updateEventStatus, Description: "Update alert event status"},
		},
	}
	eventGroup.Register(ginEngine.Group(alertEventBaseURL), r.c.APIResource())

	channelGroup := &apiregistry.Group{
		Name:    "告警",
		BaseURL: alertChannelBaseURL,
		Entries: []apiregistry.RouteEntry{
			{Method: "POST", RelativePath: "", Handler: r.createChannel, Description: "Create alert channel"},
			{Method: "PUT", RelativePath: "/:channelId", Handler: r.updateChannel, Description: "Update alert channel"},
			{Method: "DELETE", RelativePath: "/:channelId", Handler: r.deleteChannel, Description: "Delete alert channel"},
			{Method: "GET", RelativePath: "", Handler: r.listChannels, Description: "List alert channels"},
			{Method: "GET", RelativePath: "/:channelId", Handler: r.getChannel, Description: "Get alert channel"},
			{Method: "POST", RelativePath: "/ping", Handler: r.pingChannel, Description: "Ping alert channel connectivity"},
		},
	}
	channelGroup.Register(ginEngine.Group(alertChannelBaseURL), r.c.APIResource())

	notifyGroup := &apiregistry.Group{
		Name:    "告警",
		BaseURL: alertNotifyBaseURL,
		Entries: []apiregistry.RouteEntry{
			{Method: "GET", RelativePath: "", Handler: r.listNotifications, Description: "List alert notification records"},
		},
	}
	notifyGroup.Register(ginEngine.Group(alertNotifyBaseURL), r.c.APIResource())

	silenceGroup := &apiregistry.Group{
		Name:    "告警",
		BaseURL: alertSilenceBaseURL,
		Entries: []apiregistry.RouteEntry{
			{Method: "POST", RelativePath: "", Handler: r.createSilence, Description: "Create alert silence"},
			{Method: "PUT", RelativePath: "/:silenceId", Handler: r.updateSilence, Description: "Update alert silence"},
			{Method: "DELETE", RelativePath: "/:silenceId", Handler: r.deleteSilence, Description: "Delete alert silence"},
			{Method: "GET", RelativePath: "", Handler: r.listSilences, Description: "List alert silences"},
			{Method: "GET", RelativePath: "/:silenceId", Handler: r.getSilence, Description: "Get alert silence"},
		},
	}
	silenceGroup.Register(ginEngine.Group(alertSilenceBaseURL), r.c.APIResource())
}
