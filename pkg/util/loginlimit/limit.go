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
	// 同一 IP 每分钟最多 10 次登录尝试（突发 5）
	ipRate  = rate.Limit(10.0 / 60.0)
	ipBurst = 5

	// 同一用户名全局每分钟最多 10 次（防多 IP 打同一账号）
	userRate  = rate.Limit(10.0 / 60.0)
	userBurst = 5

	// 同时进行的 bcrypt 校验上限，避免 CPU 被打满
	maxConcurrentVerify = 4
	acquireWait         = 50 * time.Millisecond
)

var (
	limiters  sync.Map // map[string]*rate.Limiter
	verifySem = make(chan struct{}, maxConcurrentVerify)
)

func getLimiter(key string, limit rate.Limit, burst int) *rate.Limiter {
	if v, ok := limiters.Load(key); ok {
		return v.(*rate.Limiter)
	}
	lim := rate.NewLimiter(limit, burst)
	actual, _ := limiters.LoadOrStore(key, lim)
	return actual.(*rate.Limiter)
}

// AllowIP 按客户端 IP 限流登录请求。
func AllowIP(ip string) bool {
	if ip == "" {
		ip = "unknown"
	}
	return getLimiter("ip:"+ip, ipRate, ipBurst).Allow()
}

// AllowUser 按用户名全局限流（不区分 IP），在 bcrypt 之前调用。
func AllowUser(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return false
	}
	return getLimiter("user:"+name, userRate, userBurst).Allow()
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
