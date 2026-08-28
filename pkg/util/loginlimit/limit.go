/*
Copyright 2024 The Pixiu Authors.

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

package loginlimit

import (
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	// 全局限流：所有 IP 合计，优先挡住分布式刷登录
	globalRate  = rate.Limit(20)
	globalBurst = 40

	// 同一 IP 每分钟最多 5 次（突发 3）
	ipRate  = rate.Limit(5.0 / 60.0)
	ipBurst = 3

	// 密码连续失败达到阈值后锁定；锁定期间每分钟仅允许 1 次探测（给正确密码留机会）
	maxFailures      = 8
	lockDuration     = 2 * time.Minute
	lockedProbeRate  = rate.Limit(1.0 / 60.0)
	lockedProbeBurst = 1

	// 同时进行的 bcrypt 校验上限，避免 CPU 被打满
	maxConcurrentVerify = 4
	acquireWait         = 50 * time.Millisecond
)

var (
	limiters  sync.Map // map[string]*rate.Limiter
	verifySem = make(chan struct{}, maxConcurrentVerify)

	globalLimiter = rate.NewLimiter(globalRate, globalBurst)

	failureMu sync.Mutex
	failures  = map[string]*failureState{}
)

type failureState struct {
	count       int
	lockedUntil time.Time
}

func getLimiter(key string, limit rate.Limit, burst int) *rate.Limiter {
	if v, ok := limiters.Load(key); ok {
		return v.(*rate.Limiter)
	}
	lim := rate.NewLimiter(limit, burst)
	actual, _ := limiters.LoadOrStore(key, lim)
	return actual.(*rate.Limiter)
}

func normalizeUser(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// AllowRequest 登录入口限流：先全局 QPS，再按 IP。
func AllowRequest(ip string) bool {
	if !globalLimiter.Allow() {
		return false
	}
	if ip == "" {
		ip = "unknown"
	}
	return getLimiter("ip:"+ip, ipRate, ipBurst).Allow()
}

// AllowIP 按客户端 IP 限流（兼容旧调用，等价于无全局限流时的 IP 检查）。
func AllowIP(ip string) bool {
	return AllowRequest(ip)
}

// AllowUserAttempt 用户名维度：未锁定直接放行；锁定后每分钟仅 1 次 bcrypt 探测。
func AllowUserAttempt(name string) bool {
	name = normalizeUser(name)
	if name == "" {
		return false
	}

	failureMu.Lock()
	st := failures[name]
	locked := st != nil && time.Now().Before(st.lockedUntil)
	failureMu.Unlock()

	if !locked {
		return true
	}
	return getLimiter("locked-probe:"+name, lockedProbeRate, lockedProbeBurst).Allow()
}

// RecordUserFailure 记录密码错误；达到阈值则锁定一段时间。
func RecordUserFailure(name string) {
	name = normalizeUser(name)
	if name == "" {
		return
	}

	now := time.Now()
	failureMu.Lock()
	defer failureMu.Unlock()

	st := failures[name]
	if st == nil {
		st = &failureState{}
		failures[name] = st
	}
	if now.After(st.lockedUntil) && st.count >= maxFailures {
		// 锁已过期，重新计数
		st.count = 0
	}
	st.count++
	if st.count >= maxFailures {
		st.lockedUntil = now.Add(lockDuration)
	}
}

// ClearUserFailures 登录成功后清除失败计数与锁定。
func ClearUserFailures(name string) {
	name = normalizeUser(name)
	if name == "" {
		return
	}
	failureMu.Lock()
	delete(failures, name)
	failureMu.Unlock()
}

// AcquireVerify 获取密码校验名额；瞬时打满时短暂等待，仍无空闲则失败。
func AcquireVerify() bool {
	select {
	case verifySem <- struct{}{}:
		return true
	default:
	}
	timer := time.NewTimer(acquireWait)
	defer timer.Stop()
	select {
	case verifySem <- struct{}{}:
		return true
	case <-timer.C:
		return false
	}
}

// ReleaseVerify 释放密码校验名额。
func ReleaseVerify() {
	select {
	case <-verifySem:
	default:
	}
}
