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

package jobmanager

import (
	"sync"
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

// 指数退避：30s 基数，每多一次连续失败间隔翻倍，封顶 480s(8min)，成功重置。
const (
	backoffBaseInterval = 30 * time.Second
	backoffMaxInterval  = 8 * time.Minute
	backoffMaxFailures  = 8

	// maxTextLen 探测信息、审计详情等字段的长度上限（对应 varchar(512)）。
	maxTextLen = 500

	// disconnectConfirmFailures 连续探测失败多少次才判定失联。
	// 首次失败多为抖动，单次失败即置失联会产生大量"失联-恢复"噪音审计。
	disconnectConfirmFailures = 2
)

// 探测结论，写入集群 probe_reason 并参与失联审计的文案判定。
const (
	probeReasonHealthy           = "Healthy"
	probeReasonProbeFailed       = "ProbeFailed"
	probeReasonAgentDisconnected = "AgentDisconnected"
)

// probeBackoff 集群连通性探测的并发安全指数退避器。
// 以集群名为 key 分别记录连续失败次数与上次探测时间。
type probeBackoff struct {
	mu          sync.Mutex
	failures    map[string]int
	lastAttempt map[string]time.Time
}

func newProbeBackoff() *probeBackoff {
	return &probeBackoff{
		failures:    make(map[string]int),
		lastAttempt: make(map[string]time.Time),
	}
}

// probeInterval 返回连续 failures 次失败后的探测间隔。
// interval(failures) = min(backoffBaseInterval << min(failures,4), backoffMaxInterval)
func probeInterval(failures int) time.Duration {
	if failures > 4 {
		failures = 4
	}
	d := backoffBaseInterval << failures
	if d > backoffMaxInterval {
		return backoffMaxInterval
	}
	return d
}

// shouldProbe 返回是否应探测：距上次尝试 >= 当前退避间隔则 true，并刷新 lastAttempt。
func (p *probeBackoff) shouldProbe(name string, now time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	last := p.lastAttempt[name]
	if last.IsZero() || now.Sub(last) >= probeInterval(p.failures[name]) {
		p.lastAttempt[name] = now
		return true
	}
	return false
}

// markResult 记录探测结果并返回该集群当前的连续失败次数：
// success=true 清空失败计数，否则 +1（封顶 backoffMaxFailures）。
func (p *probeBackoff) markResult(name string, success bool, _ time.Time) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if success {
		delete(p.failures, name)
		return 0
	}
	f := p.failures[name] + 1
	if f > backoffMaxFailures {
		f = backoffMaxFailures
	}
	p.failures[name] = f
	return f
}

// cleanup 删除不存在集群的退避状态，避免集群删除后状态残留。
func (p *probeBackoff) cleanup(exist map[string]struct{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for name := range p.failures {
		if _, ok := exist[name]; !ok {
			delete(p.failures, name)
		}
	}
	for name := range p.lastAttempt {
		if _, ok := exist[name]; !ok {
			delete(p.lastAttempt, name)
		}
	}
}

// truncateText 按字符截断文本，避免超出 varchar(512) 导致整条事务回滚。
func truncateText(text string) string {
	runes := []rune(text)
	if len(runes) <= maxTextLen {
		return text
	}
	return string(runes[:maxTextLen])
}

// probeConditionUpdates 构建集群连通性 condition 的写库字段。
// last_probe_time 始终刷新；last_healthy_time 只在探测成功时刷新，用于推算失联起点。
func probeConditionUpdates(connected bool, reason, message string, now time.Time) map[string]interface{} {
	status := model.ClusterProbeUnhealthy
	if connected {
		status = model.ClusterProbeHealthy
	}
	updates := map[string]interface{}{
		"probe_status":    status,
		"probe_reason":    reason,
		"probe_message":   truncateText(message),
		"last_probe_time": now.Truncate(time.Microsecond),
	}
	if connected {
		updates["last_healthy_time"] = now.Truncate(time.Microsecond)
	}
	return updates
}
