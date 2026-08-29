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

	"github.com/caoyingjunz/pixiu/pkg/util/lru"
)

const (
	// 全局限流：所有 IP 合计，优先挡住分布式刷登录
	globalRate  = rate.Limit(20)
	globalBurst = 40

	// 同一 IP 每分钟最多 5 次（突发 3）
	ipRate  = rate.Limit(5.0 / 60.0)
	ipBurst = 3

	// 密码连续失败达到阈值后锁定；锁定期间每分钟仅允许 1 次探测
	maxFailures      = 8
	lockDuration     = 2 * time.Minute
	lockedProbeRate  = rate.Limit(1.0 / 60.0)
	lockedProbeBurst = 1

	// 同时进行的 bcrypt 校验上限
	maxConcurrentVerify = 4
	acquireWait         = 50 * time.Millisecond

	// IP / 探测 limiter 与失败记录的容量与 TTL，防止内存膨胀
	ipLimiterCap      = 8192
	probeLimiterCap   = 4096
	maxFailureEntries = 4096
	entryTTL          = 30 * time.Minute
	purgeInterval     = 5 * time.Minute
)

var (
	ipLimiters    = lru.NewLRUCache(ipLimiterCap)
	probeLimiters = lru.NewLRUCache(probeLimiterCap)
	verifySem     = make(chan struct{}, maxConcurrentVerify)
	globalLimiter = rate.NewLimiter(globalRate, globalBurst)

	failureMu sync.Mutex
	failures  = map[string]*failureState{}

	sweeperOnce sync.Once
)

type failureState struct {
	count       int
	lockedUntil time.Time
	lastSeen    time.Time
}

func ensureSweeper() {
	sweeperOnce.Do(func() {
		go func() {
			t := time.NewTicker(purgeInterval)
			defer t.Stop()
			for range t.C {
				purgeExpiredFailures(time.Now())
			}
		}()
	})
}

func purgeExpiredFailures(now time.Time) {
	failureMu.Lock()
	defer failureMu.Unlock()
	for name, st := range failures {
		if now.Before(st.lockedUntil) {
			continue
		}
		if now.Sub(st.lastSeen) > entryTTL {
			delete(failures, name)
		}
	}
	// 仍超限时按 lastSeen 淘汰最旧条目
	for len(failures) > maxFailureEntries {
		var oldest string
		var oldestTime time.Time
		first := true
		for name, st := range failures {
			if now.Before(st.lockedUntil) {
				continue
			}
			if first || st.lastSeen.Before(oldestTime) {
				oldest = name
				oldestTime = st.lastSeen
				first = false
			}
		}
		if oldest == "" {
			break
		}
		delete(failures, oldest)
	}
}

func normalizeUser(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func getRateLimiter(cache *lru.LRUCache, key string, limit rate.Limit, burst int) *rate.Limiter {
	return cache.GetOrAdd(key, func() interface{} {
		return rate.NewLimiter(limit, burst)
	}).(*rate.Limiter)
}

// AllowRequest 登录入口限流：先全局 QPS，再按 IP。
func AllowRequest(ip string) bool {
	ensureSweeper()
	if !globalLimiter.Allow() {
		return false
	}
	if ip == "" {
		ip = "unknown"
	}
	return getRateLimiter(ipLimiters, "ip:"+ip, ipRate, ipBurst).Allow()
}

// AllowIP 兼容旧调用。
func AllowIP(ip string) bool {
	return AllowRequest(ip)
}

// AllowUserAttempt 用户名维度：未锁定直接放行；锁定后每分钟仅 1 次 bcrypt 探测。
func AllowUserAttempt(name string) bool {
	ensureSweeper()
	name = normalizeUser(name)
	if name == "" {
		return false
	}

	failureMu.Lock()
	st := failures[name]
	locked := st != nil && time.Now().Before(st.lockedUntil)
	if st != nil {
		st.lastSeen = time.Now()
	}
	failureMu.Unlock()

	if !locked {
		return true
	}
	return getRateLimiter(probeLimiters, "locked-probe:"+name, lockedProbeRate, lockedProbeBurst).Allow()
}

// RecordUserFailure 记录密码错误；达到阈值则锁定一段时间。
func RecordUserFailure(name string) {
	ensureSweeper()
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
		st.count = 0
	}
	st.count++
	st.lastSeen = now
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

// FailureEntryCount 供测试读取失败表大小。
func FailureEntryCount() int {
	failureMu.Lock()
	defer failureMu.Unlock()
	return len(failures)
}
