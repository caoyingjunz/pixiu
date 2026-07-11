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
	"context"
	"fmt"
	"strconv"

	"github.com/robfig/cron/v3"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

type RuleWorker struct {
	ruleID  int64
	hash    string
	manager *Manager
	cron    *cron.Cron
}

func NewRuleWorker(rule *model.AlertRule, manager *Manager) (*RuleWorker, error) {
	ruleCopy := *rule
	worker := &RuleWorker{
		ruleID:  ruleCopy.Id,
		hash:    ruleWorkerHash(&ruleCopy),
		manager: manager,
	}

	cronSpec := EvalCronSpec(ruleCopy.EvalInterval)
	worker.cron = cron.New(cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)))
	if _, err := worker.cron.AddFunc(cronSpec, func() {
		if err := manager.EvaluateRule(context.Background(), &ruleCopy); err != nil {
			klog.Errorf("failed to evaluate alert rule(%d:%s): %v", ruleCopy.Id, ruleCopy.Name, err)
		}
	}); err != nil {
		return nil, fmt.Errorf("failed to add cron for alert rule(%d): %w", ruleCopy.Id, err)
	}
	return worker, nil
}

func (w *RuleWorker) Start() {
	w.cron.Start()
}

func (w *RuleWorker) Stop() {
	ctx := w.cron.Stop()
	<-ctx.Done()
}

func ruleWorkerHash(rule *model.AlertRule) string {
	return strconv.FormatInt(rule.Id, 10) + "_" +
		strconv.FormatInt(rule.ResourceVersion, 10) + "_" +
		strconv.Itoa(NormalizeEvalInterval(rule.EvalInterval)) + "_" +
		strconv.FormatBool(rule.Enabled) + "_" +
		rule.MetricName + "_" +
		rule.ConditionExpr + "_" +
		strconv.Itoa(int(rule.RuleType)) + "_" +
		strconv.Itoa(rule.Duration) + "_" +
		strconv.Itoa(int(rule.Severity)) + "_" +
		strconv.Itoa(int(rule.ScopeType)) + "_" +
		rule.ScopeValue + "_" +
		rule.NotifyChannels
}
