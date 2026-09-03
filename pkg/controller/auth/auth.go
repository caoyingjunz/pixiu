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

func (c *controller) SendCode(ctx context.Context, req *types.SendRegistrationCodeRequest, requestIP string) (*types.RegistrationCodeResponse, error) {
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
		return c.storeRegistrationCode(ctx, factory, object, codeCooldown)
	})
	if err != nil {
		if mapped := mapRegistrationError(err); mapped != err {
			return nil, mapped
		}
		klog.Errorf("failed to store registration code for %s: %v", email, err)
		return nil, apierrors.ErrServerInternal
	}

	subject := "Pixiu 注册验证码"
	body := fmt.Sprintf("您正在注册 Pixiu 账号。\n\n验证码：%s\n\n验证码 %d 分钟内有效，请勿转发给他人。", code, int(codeTTL/time.Minute))
	if err = emailcontroller.New(c.cc, c.factory).Send(ctx, email, subject, body); err != nil {
		if invalidateErr := c.factory.Auth().InvalidateCode(ctx, email, codeHash); invalidateErr != nil {
			klog.Errorf("failed to invalidate unsent registration code for %s: %v", email, invalidateErr)
		}
		klog.Errorf("failed to send registration code to %s: %v", email, err)
		return nil, apierrors.ErrRegistrationEmailUnavailable
	}

	return &types.RegistrationCodeResponse{
		ExpiresIn:  int(codeTTL / time.Second),
		RetryAfter: int(codeCooldown / time.Second),
	}, nil
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
