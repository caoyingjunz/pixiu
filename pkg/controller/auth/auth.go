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
	goerrors "errors"
	"fmt"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"

	"k8s.io/klog/v2"

	"github.com/gin-gonic/gin"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	emailcontroller "github.com/caoyingjunz/pixiu/pkg/controller/email"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/caoyingjunz/pixiu/pkg/util"
	"github.com/caoyingjunz/pixiu/pkg/util/loginlimit"
)

const (
	codeDigits       = 6
	codeTTL          = 5 * time.Minute
	codeCooldown     = time.Minute
	maxCodeAttempts  = 5
	minUsernameRunes = 3
	maxUsernameRunes = 20
	minPasswordRunes = 6

	// 验证码场景：注册 / 忘记密码。存于 registration_codes.scene，与 email 组成联合唯一索引。
	sceneRegister = "register"
	sceneForgot   = "forgot"
	// 忘记密码防枚举冷却占位场景：独立于真实发码场景（register/forgot），既避免覆盖真实验证码，
	// 也保证 resetPassword 按 sceneForgot 精确查询时永远读不到占位行。注意 scene 列 varchar(16)。
	sceneForgotCooldown = "forgot_cool"
	// placeholderCodeHash 占位记录的 code_hash：非 64 位十六进制，任何 codeHash() 产物都不可能与之相等。
	placeholderCodeHash = "cooldown-placeholder"
)

type Getter interface {
	Auth() Interface
}

type Interface interface {
	SendVerificationCode(ctx context.Context, req *types.SendRegistrationCodeRequest, requestIP string) (*types.RegistrationCodeResponse, error)
	Register(ctx context.Context, req *types.RegisterUserRequest) error
	SendForgotPasswordCode(ctx context.Context, req *types.SendForgotPasswordCodeRequest, requestIP string) (*types.RegistrationCodeResponse, error)
	ResetPassword(ctx context.Context, req *types.ResetPasswordRequest) error
	Login(ctx context.Context, req *types.LoginRequest) (*types.LoginResponse, error)
	Logout(ctx *gin.Context) error
	Refresh(ctx context.Context) error
	ValidateLoginToken(ctx context.Context, userId int64, token string) (bool, error)
	GetLoginToken(ctx context.Context, userId int64) (string, error)
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

func (c *controller) SendVerificationCode(ctx context.Context, req *types.SendRegistrationCodeRequest, requestIP string) (*types.RegistrationCodeResponse, error) {
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
		Scene:          sceneRegister,
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
		if expireErr := c.expireUnsentRegistrationCode(ctx, c.factory, email, codeHash, sceneRegister); expireErr != nil {
			klog.Errorf("failed to expire unsent registration code for %s: %v", email, expireErr)
		}
		klog.Errorf("failed to send registration code to %s: %v", email, err)
		return nil, apierrors.ErrRegistrationEmailUnavailable
	}

	return &types.RegistrationCodeResponse{ExpiresIn: int(codeTTL / time.Second), RetryAfter: int(codeCooldown / time.Second)}, nil
}

// SendForgotPasswordCode 发送忘记密码验证码。
// 与注册发码共用 preSendCode 默认邮件配置校验、邮箱归一化、6 位码 + HMAC 入库、冷却覆盖逻辑，
// 场景标记为 forgot；并按防枚举要求：邮箱未注册时返回与成功相同的响应，不发码、不发邮件，
// 但登记一条独立 scene 的冷却占位（forgot_cool），使两条路径的重复发码冷却行为一致。
func (c *controller) SendForgotPasswordCode(ctx context.Context, req *types.SendForgotPasswordCodeRequest, requestIP string) (*types.RegistrationCodeResponse, error) {
	if err := c.preSendCode(ctx); err != nil {
		return nil, err
	}

	email := normalizeEmail(req.Email)
	if email == "" {
		return nil, apierrors.ErrInvalidRequest
	}

	// 防枚举：仅当邮箱已注册才真正发码；未注册时统一返回成功响应，避免泄漏邮箱注册状态
	user, err := c.factory.User().GetBy(ctx, db.WithEmail(email))
	if err != nil {
		klog.Errorf("failed to check forgot password email: %v", err)
		return nil, apierrors.ErrServerInternal
	}
	if user == nil {
		// 未注册邮箱不发码，但仍登记冷却占位：使「已注册/未注册」两条路径对同一邮箱的重复请求
		// 冷却行为一致（否则连发两次即可判定注册状态），闭合「静默成功」防枚举的冷却窗口 oracle。
		if err := c.issueCooldownPlaceholder(ctx, email, sceneForgotCooldown, requestIP); err != nil {
			if mapped := mapRegistrationError(err); mapped != err {
				return nil, mapped
			}
			klog.Errorf("failed to issue forgot password cooldown placeholder for %s: %v", email, err)
			return nil, apierrors.ErrServerInternal
		}
		return &types.RegistrationCodeResponse{ExpiresIn: int(codeTTL / time.Second), RetryAfter: int(codeCooldown / time.Second)}, nil
	}

	code, err := generateCode()
	if err != nil {
		klog.Errorf("failed to generate forgot password code: %v", err)
		return nil, apierrors.ErrServerInternal
	}

	now := time.Now()
	codeHash := c.codeHash(email, code)
	object := &model.RegistrationCode{
		Email:          email,
		Scene:          sceneForgot,
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
		klog.Errorf("failed to store forgot password code for %s: %v", email, err)
		return nil, apierrors.ErrServerInternal
	}

	subject := "PixiuCloud密码重置"
	body := fmt.Sprintf("【PixiuCloud】您正在重置密码，验证码为：%s，有效期为 %d 分钟，如非本人操作请忽略。", code, int(codeTTL/time.Minute))
	if err = emailcontroller.New(c.cc, c.factory).Send(ctx, email, subject, body); err != nil {
		if expireErr := c.expireUnsentRegistrationCode(ctx, c.factory, email, codeHash, sceneForgot); expireErr != nil {
			klog.Errorf("failed to expire unsent forgot password code for %s: %v", email, expireErr)
		}
		klog.Errorf("failed to send forgot password code to %s: %v", email, err)
		return nil, apierrors.ErrRegistrationEmailUnavailable
	}

	return &types.RegistrationCodeResponse{ExpiresIn: int(codeTTL / time.Second), RetryAfter: int(codeCooldown / time.Second)}, nil
}

// ResetPassword 忘记密码重置密码。
// 校验忘记密码验证码（仅消费 forgot 场景、未用/未过期/未超次/常量时间比对），
// 通过后更新该用户密码并在同一事务内原子标记验证码已用。
// 重置成功后吊销该用户全部旧 token，并解除其登录失败锁定（内存态按用户名锁定）。
func (c *controller) ResetPassword(ctx context.Context, req *types.ResetPasswordRequest) error {
	email := normalizeEmail(req.Email)
	code := strings.TrimSpace(req.Code)
	if email == "" {
		return apierrors.ErrInvalidRequest
	}
	if len(code) != codeDigits {
		return apierrors.ErrRegistrationCodeInvalid
	}
	if utf8.RuneCountInString(req.NewPassword) < minPasswordRunes {
		return apierrors.ErrInvalidRequest
	}

	// 防枚举兜底：邮箱不存在时统一返回验证码错误，不泄漏邮箱注册状态
	user, err := c.factory.User().GetBy(ctx, db.WithEmail(email))
	if err != nil {
		klog.Errorf("failed to query user for password reset: %v", err)
		return apierrors.ErrServerInternal
	}
	if user == nil {
		return apierrors.ErrRegistrationCodeInvalid
	}

	encrypted, err := util.EncryptUserPassword(req.NewPassword)
	if err != nil {
		klog.Errorf("failed to encrypt reset password: %v", err)
		return apierrors.ErrServerInternal
	}

	// 猜码失败（errCodeInvalid/errCodeAttempts）时提交事务以保留 failed_attempts 计数，
	// 否则事务回滚会丢失锁定进度，5 次错误锁定形同虚设；其余错误（过期/并发消费等）照常回滚。
	var innerErr error
	if err = c.factory.Transaction(ctx, func(factory db.ShareDaoFactory) error {
		innerErr = c.resetPassword(ctx, factory, email, c.codeHash(email, code), encrypted)
		if goerrors.Is(innerErr, errCodeInvalid) || goerrors.Is(innerErr, errCodeAttempts) {
			return nil
		}
		return innerErr
	}); err != nil {
		if mapped := mapRegistrationError(err); mapped != err {
			return mapped
		}
		klog.Errorf("failed to reset password for %s: %v", email, err)
		return apierrors.ErrServerInternal
	}
	if innerErr != nil {
		if mapped := mapRegistrationError(innerErr); mapped != innerErr {
			return mapped
		}
		klog.Errorf("failed to reset password for %s: %v", email, innerErr)
		return apierrors.ErrServerInternal
	}

	// 重置成功后吊销该用户全部旧 token，旧登录态立即失效；并解除该用户名的登录失败锁定
	RevokeUserTokens(user.Id)
	loginlimit.ClearUserFailures(user.Name)
	return nil
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
