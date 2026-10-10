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

package auth

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/util"
)

type oauthStateRecord struct {
	Provider  string
	SessionID string
	ExpireAt  time.Time
}

// oauthStates / oauthIdentityLocks 为进程内状态：state 会话与去重锁仅在单实例内有效，
// 多实例部署需外部共享存储（如 Redis）承载，否则跨实例回跳可能校验失败。
var (
	oauthStates        sync.Map
	oauthIdentityLocks sync.Map
)

func (c *controller) oauthState(ctx context.Context, provider string) (string, error) {
	sessionID := oauthSessionID(ctx)
	if sessionID == "" {
		return "", fmt.Errorf("第三方登录会话初始化失败，请刷新页面后重试")
	}
	state := util.RandomHex(32)
	oauthStates.Store(state, oauthStateRecord{
		Provider:  provider,
		SessionID: sessionID,
		ExpireAt:  time.Now().Add(oauthStateTTL),
	})
	purgeExpiredOAuthStates()
	return state, nil
}

func (c *controller) validateOAuthState(ctx context.Context, provider, state string) bool {
	sessionID := oauthSessionID(ctx)
	if sessionID == "" || strings.TrimSpace(state) == "" {
		return false
	}
	value, ok := oauthStates.LoadAndDelete(state)
	if !ok {
		return false
	}
	record := value.(oauthStateRecord)
	return record.Provider == provider &&
		record.SessionID == sessionID &&
		time.Now().Before(record.ExpireAt)
}

func oauthSessionID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	sessionID, _ := ctx.Value(oauthSessionIDContextKey{}).(string)
	return strings.TrimSpace(sessionID)
}

func purgeExpiredOAuthStates() {
	now := time.Now()
	oauthStates.Range(func(key, value interface{}) bool {
		record := value.(oauthStateRecord)
		if now.After(record.ExpireAt) {
			oauthStates.Delete(key)
		}
		return true
	})
}

// oauthIdentityLockKey 计算建号/绑定去重锁的 key：
// 当启用邮箱匹配且邮箱非空时，以「provider+归一化邮箱」为 key，序列化同邮箱并发建号；
// 否则回退到 union/open id（无可用标识时返回空，不加锁）。
func oauthIdentityLockKey(cfg *model.OAuthProvider, provider string, profile *oauthUserProfile, email string) string {
	if cfg != nil && cfg.MatchEmail && strings.TrimSpace(email) != "" {
		return provider + ":email:" + normalizeEmail(email)
	}
	if profile == nil {
		return ""
	}
	if profile.UnionID != "" {
		return provider + ":union:" + profile.UnionID
	}
	if profile.OpenID != "" {
		return provider + ":open:" + profile.OpenID
	}
	return ""
}

func getOAuthIdentityLock(key string) *sync.Mutex {
	lock, _ := oauthIdentityLocks.LoadOrStore(key, &sync.Mutex{})
	return lock.(*sync.Mutex)
}
