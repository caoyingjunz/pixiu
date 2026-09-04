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
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"

	"k8s.io/klog/v2"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	emailcontroller "github.com/caoyingjunz/pixiu/pkg/controller/email"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/caoyingjunz/pixiu/pkg/util"
)

const (
	codeDigits       = 6
	codeTTL          = 5 * time.Minute
	codeCooldown     = time.Minute
	maxCodeAttempts  = 5
	minUsernameRunes = 3
	maxUsernameRunes = 20
)

type Getter interface {
	Auth() Interface
}

type Interface interface {
	SendCode(ctx context.Context, req *types.SendRegistrationCodeRequest, requestIP string) (*types.RegistrationCodeResponse, error)
	Register(ctx context.Context, req *types.RegisterUserRequest) error
}

type controller struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

// preSendCode 发码前置检查：未配置默认启用系统邮箱时直接失败（fail fast），
// 避免白生成验证码并落库后再作废。
func (c *controller) preSendCode(ctx context.Context) error {
	defaultEmail, err := c.factory.Email().GetBy(ctx,
		db.WithEnabled(true), db.WithIsDefault(true), db.WithOrderByDesc())
	if err != nil {
		klog.Errorf("failed to check default email config: %v", err)
		return apierrors.ErrServerInternal
	}
	if defaultEmail == nil {
		return apierrors.ErrEmailNotConfigured
	}
	return nil
}

func (c *controller) SendCode(ctx context.Context, req *types.SendRegistrationCodeRequest, requestIP string) (*types.RegistrationCodeResponse, error) {
	if err := c.preSendCode(ctx); err != nil {
		return nil, err
	}

	email := normalizeEmail(req.Email)
	existing, err := c.factory.User().GetBy(ctx, db.WithEmail(email))
	if err != nil {
		klog.Errorf("failed to check registration email: %v", err)
		return nil, apierrors.ErrServerInternal
	}
	if existing != nil {
		return nil, apierrors.ErrEmailExists
	}

	code, err := generateCode()
	if err != nil {
		klog.Errorf("failed to generate registration code: %v", err)
		return nil, apierrors.ErrServerInternal
	}

	now := time.Now()
	codeHash := c.codeHash(email, code)
	object := &model.RegistrationCode{
		Email:          email,
		CodeHash:       codeHash,
		ExpiresAt:      now.Add(codeTTL),
		FailedAttempts: 0,
		SentAt:         now,
		RequestIP:      requestIP,
	}
	err = c.factory.Transaction(ctx, func(factory db.ShareDaoFactory) error {
		return c.issueRegistrationCode(ctx, factory, object, codeCooldown)
	})
	if err != nil {
		if mapped := mapRegistrationError(err); mapped != err {
			return nil, mapped
		}
		klog.Errorf("failed to store registration code for %s: %v", email, err)
		return nil, apierrors.ErrServerInternal
	}

	subject := "PixiuCloud账号激活"
	body := fmt.Sprintf("【PixiuCloud】亲爱的用户，您的注册验证码为：%s，有效期为 %d 分钟，如非本人操作请忽略。", code, int(codeTTL/time.Minute))
	if err = emailcontroller.New(c.cc, c.factory).Send(ctx, email, subject, body); err != nil {
		if expireErr := c.expireUnsentRegistrationCode(ctx, c.factory, email, codeHash); expireErr != nil {
			klog.Errorf("failed to expire unsent registration code for %s: %v", email, expireErr)
		}
		klog.Errorf("failed to send registration code to %s: %v", email, err)
		return nil, apierrors.ErrRegistrationEmailUnavailable
	}

	return &types.RegistrationCodeResponse{ExpiresIn: int(codeTTL / time.Second), RetryAfter: int(codeCooldown / time.Second)}, nil
}

func (c *controller) Register(ctx context.Context, req *types.RegisterUserRequest) error {
	name := strings.TrimSpace(req.Name)
	if count := utf8.RuneCountInString(name); count < minUsernameRunes || count > maxUsernameRunes {
		return apierrors.ErrInvalidRequest
	}
	email := normalizeEmail(req.Email)
	code := strings.TrimSpace(req.Code)

	encrypted, err := util.EncryptUserPassword(req.Password)
	if err != nil {
		klog.Errorf("failed to encrypt registration password: %v", err)
		return apierrors.ErrServerInternal
	}
	user := &model.User{
		Name:     name,
		Password: encrypted,
		Status:   model.UserStatusNormal,
		Email:    email,
	}
	err = c.factory.Transaction(ctx, func(factory db.ShareDaoFactory) error {
		return c.registerUser(ctx, factory, email, c.codeHash(email, code), user)
	})
	if err != nil {
		if mapped := mapRegistrationError(err); mapped != err {
			return mapped
		}
		klog.Errorf("failed to register user %s: %v", name, err)
		return apierrors.ErrServerInternal
	}
	return nil
}

func (c *controller) codeHash(email, code string) string {
	key := sha256.Sum256([]byte("pixiu-registration-code:" + c.cc.Default.JWTKey))
	mac := hmac.New(sha256.New, key[:])
	_, _ = mac.Write([]byte(email))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil))
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func generateCode() (string, error) {
	max := big.NewInt(1_000_000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", codeDigits, n.Int64()), nil
}

func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &controller{cc: cfg, factory: f}
}
