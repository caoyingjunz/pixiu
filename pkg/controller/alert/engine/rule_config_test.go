/*
Copyright 2026 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

    10|Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package engine

import (
	"testing"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

func TestNormalizeRuleConfigTriggers(t *testing.T) {
	raw := `{"prom_ql":"up","triggers":[{"severity":1,"condition":">= 0","duration":120},{"severity":2,"condition":"> 80","duration":60}]}`
	cfg, sev, duration, ok := NormalizeRuleConfig(raw, model.AlertSeverityWarning)
	if !ok || cfg == "" {
		t.Fatal("expected config")
	}
	if sev != model.AlertSeverityInfo {
		t.Fatalf("got sev=%d", sev)
	}
	if duration != 120 {
		t.Fatalf("got duration=%d", duration)
	}
	triggers := ListAlertTriggers(&model.AlertRule{RuleConfig: cfg})
	if len(triggers) != 2 {
		t.Fatalf("want 2 triggers got %d", len(triggers))
	}
	if triggers[0].PromQl != "up" || triggers[0].Condition != ">= 0" {
		t.Fatalf("unexpected first trigger: %+v", triggers[0])
	}
}

func TestNormalizeRuleConfigLegacyQueries(t *testing.T) {
	raw := `{"queries":[{"prom_ql":"up","severity":1},{"prom_ql":"> 80","severity":2}]}`
	cfg, sev, _, ok := NormalizeRuleConfig(raw, model.AlertSeverityWarning)
	if !ok || cfg == "" {
		t.Fatal("expected config")
	}
	if sev != model.AlertSeverityInfo {
		t.Fatalf("got sev=%d", sev)
	}
	triggers := ListAlertTriggers(&model.AlertRule{RuleConfig: cfg})
	if len(triggers) != 2 {
		t.Fatalf("want 2 triggers got %d", len(triggers))
	}
	if triggers[0].PromQl != "up" {
		t.Fatalf("want shared prom_ql=up got %q", triggers[0].PromQl)
	}
}

func TestNormalizeRuleConfigEmpty(t *testing.T) {
	_, _, _, ok := NormalizeRuleConfig("", model.AlertSeverityCritical)
	if ok {
		t.Fatal("expected empty config to fail")
	}
}
