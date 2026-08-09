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

package emailconfig

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/smtp"
	"strconv"
	"strings"

	"k8s.io/klog/v2"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/caoyingjunz/pixiu/pkg/util/encryption"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type Getter interface {
	Email() Interface
}

type Interface interface {
	Create(ctx context.Context, req *types.CreateEmailConfigRequest) error
	Update(ctx context.Context, req *types.UpdateEmailConfigRequest) error
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (*types.EmailConfig, error)
	List(ctx context.Context, listOption types.ListOptions) (interface{}, error)
	TestSend(ctx context.Context, id int64, req *types.TestSendEmailRequest) error
}

type controller struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &controller{cc: cfg, factory: f}
}

// newCipher 构造基于配置密钥的 AES-GCM 加解密器（密码字段加密落库）。
func (c *controller) newCipher() (*encryption.Cipher, error) {
	return encryption.NewCipher(c.cc.Default.JWTKey)
}

func (c *controller) Create(ctx context.Context, req *types.CreateEmailConfigRequest) error {
	cipher, err := c.newCipher()
	if err != nil {
		klog.Errorf("failed to init email encryption cipher: %v", err)
		return apierrors.ErrServerInternal
	}
	user, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return apierrors.ErrUnauthorized
	}

	object := &model.EmailConfig{
		Name:        req.Name,
		SmtpHost:    req.SmtpHost,
		SmtpPort:    req.SmtpPort,
		Username:    req.Username,
		FromEmail:   req.FromEmail,
		FromName:    req.FromName,
		Encryption:  req.Encryption,
		Enabled:     req.Enabled,
		IsDefault:   req.IsDefault,
		Description: req.Description,
		CreatedBy:   user.Id,
	}
	if req.Password != "" {
		enc, encErr := cipher.Encrypt(req.Password)
		if encErr != nil {
			klog.Errorf("failed to encrypt smtp password: %v", encErr)
			return apierrors.ErrServerInternal
		}
		object.Password = enc
	}

	// 新建默认配置时，先清除既有默认标记保证唯一性
	if req.IsDefault {
		if err = c.factory.Email().ClearDefaultExcept(ctx, 0); err != nil {
			klog.Errorf("failed to clear default email config before create: %v", err)
			return apierrors.ErrServerInternal
		}
	}

	if _, err = c.factory.Email().Create(ctx, object); err != nil {
		klog.Errorf("failed to create email config %s: %v", req.Name, err)
		return apierrors.ErrServerInternal
	}
	return nil
}

// preUpdate 更新前置检查：资源存在。
func (c *controller) preUpdate(ctx context.Context, id int64) (*model.EmailConfig, error) {
	old, err := c.factory.Email().Get(ctx, id)
	if err != nil {
		klog.Errorf("failed to get email config(%d): %v", id, err)
		return nil, apierrors.ErrServerInternal
	}
	if old == nil {
		return nil, apierrors.NewError(fmt.Errorf("email config not found"), http.StatusNotFound)
	}
	return old, nil
}

func (c *controller) Update(ctx context.Context, req *types.UpdateEmailConfigRequest) error {
	old, err := c.preUpdate(ctx, req.Id)
	if err != nil {
		klog.Errorf("pre-update check failed for email config(%d): %v", req.Id, err)
		return err
	}

	updates := make(map[string]interface{})
	if req.Name != old.Name {
		updates["name"] = req.Name
	}
	if req.SmtpHost != old.SmtpHost {
		updates["smtp_host"] = req.SmtpHost
	}
	if req.SmtpPort != old.SmtpPort {
		updates["smtp_port"] = req.SmtpPort
	}
	if req.Username != old.Username {
		updates["username"] = req.Username
	}
	if req.FromEmail != old.FromEmail {
		updates["from_email"] = req.FromEmail
	}
	if req.FromName != old.FromName {
		updates["from_name"] = req.FromName
	}
	if req.Encryption != old.Encryption {
		updates["encryption"] = req.Encryption
	}
	if req.Enabled != old.Enabled {
		updates["enabled"] = req.Enabled
	}
	if req.Description != old.Description {
		updates["description"] = req.Description
	}
	// 密码留空表示保持原密码不变，仅在传入新密码时重新加密
	if req.Password != "" {
		cipher, cipherErr := c.newCipher()
		if cipherErr != nil {
			klog.Errorf("failed to init email encryption cipher: %v", cipherErr)
			return apierrors.ErrServerInternal
		}
		enc, encErr := cipher.Encrypt(req.Password)
		if encErr != nil {
			klog.Errorf("failed to encrypt smtp password: %v", encErr)
			return apierrors.ErrServerInternal
		}
		updates["password"] = enc
	}
	// IsDefault 从 false → true 时，先清除其他默认标记保证唯一性
	if req.IsDefault && !old.IsDefault {
		if err = c.factory.Email().ClearDefaultExcept(ctx, req.Id); err != nil {
			klog.Errorf("failed to clear default email config before update(%d): %v", req.Id, err)
			return apierrors.ErrServerInternal
		}
	}
	if req.IsDefault != old.IsDefault {
		updates["is_default"] = req.IsDefault
	}

	if len(updates) == 0 {
		klog.V(2).Infof("email config(%d): no fields to update", req.Id)
		return nil
	}
	if err = c.factory.Email().Update(ctx, req.Id, req.ResourceVersion, updates); err != nil {
		if utilerrors.IsNotUpdated(err) {
			return apierrors.NewError(
				fmt.Errorf("email config not found or resource version conflict"),
				http.StatusConflict,
			)
		}
		klog.Errorf("failed to update email config %d: %v", req.Id, err)
		return apierrors.ErrServerInternal
	}
	return nil
}

func (c *controller) Delete(ctx context.Context, id int64) error {
	object, err := c.factory.Email().Get(ctx, id)
	if err != nil {
		klog.Errorf("failed to get email config(%d): %v", id, err)
		return apierrors.ErrServerInternal
	}
	if object == nil {
		return apierrors.NewError(fmt.Errorf("email config not found"), http.StatusNotFound)
	}
	if _, err = c.factory.Email().Delete(ctx, id); err != nil {
		klog.Errorf("failed to delete email config %d: %v", id, err)
		return apierrors.ErrServerInternal
	}
	return nil
}

func (c *controller) Get(ctx context.Context, id int64) (*types.EmailConfig, error) {
	object, err := c.factory.Email().Get(ctx, id)
	if err != nil {
		klog.Errorf("failed to get email config(%d): %v", id, err)
		return nil, apierrors.ErrServerInternal
	}
	if object == nil {
		return nil, apierrors.NewError(fmt.Errorf("email config not found"), http.StatusNotFound)
	}
	return modelToType(object), nil
}

func (c *controller) List(ctx context.Context, listOption types.ListOptions) (interface{}, error) {
	listOption.SetDefaultPageOption()

	pageResult := types.PageResult{
		PageRequest: types.PageRequest{
			Page:  listOption.Page,
			Limit: listOption.Limit,
		},
	}

	opts := []db.Options{
		db.WithNameLike(listOption.NameSelector),
	}

	var err error
	pageResult.Total, err = c.factory.Email().Count(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to count email configs: %v", err)
		pageResult.Message = err.Error()
	}

	offset := (listOption.Page - 1) * listOption.Limit
	opts = append(opts, []db.Options{
		db.WithOrderByDesc(),
		db.WithOffset(offset),
		db.WithLimit(listOption.Limit),
	}...)

	objects, err := c.factory.Email().List(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to list email configs: %v", err)
		pageResult.Message = err.Error()
		return nil, apierrors.ErrServerInternal
	}

	items := make([]types.EmailConfig, 0)
	for i := range objects {
		items = append(items, *modelToType(&objects[i]))
	}
	pageResult.Items = items

	return pageResult, nil
}

// TestSend 使用指定配置向目标邮箱发送测试邮件，验证 SMTP 配置是否可用。
func (c *controller) TestSend(ctx context.Context, id int64, req *types.TestSendEmailRequest) error {
	object, err := c.factory.Email().Get(ctx, id)
	if err != nil {
		klog.Errorf("failed to get email config(%d): %v", id, err)
		return apierrors.ErrServerInternal
	}
	if object == nil {
		return apierrors.NewError(fmt.Errorf("email config not found"), http.StatusNotFound)
	}

	password := ""
	if object.Password != "" {
		cipher, cipherErr := c.newCipher()
		if cipherErr != nil {
			klog.Errorf("failed to init email encryption cipher: %v", cipherErr)
			return apierrors.ErrServerInternal
		}
		plain, decryptErr := cipher.Decrypt(object.Password)
		if decryptErr != nil {
			klog.Errorf("failed to decrypt smtp password for email config(%d): %v", id, decryptErr)
			return apierrors.NewError(
				fmt.Errorf("failed to decrypt smtp password, please save the config again"),
				http.StatusBadRequest,
			)
		}
		password = plain
	}

	if err = sendEmail(object, password, req.To); err != nil {
		klog.Errorf("failed to send test email via config(%d) to %s: %v", id, req.To, err)
		return apierrors.NewError(fmt.Errorf("test send email failed: %v", err), http.StatusBadRequest)
	}
	return nil
}

func modelToType(object *model.EmailConfig) *types.EmailConfig {
	return &types.EmailConfig{
		PixiuMeta: types.PixiuMeta{
			Id:              object.Id,
			ResourceVersion: object.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   object.GmtCreate,
			GmtModified: object.GmtModified,
		},
		Name:        object.Name,
		SmtpHost:    object.SmtpHost,
		SmtpPort:    object.SmtpPort,
		Username:    object.Username,
		PasswordSet: object.Password != "",
		FromEmail:   object.FromEmail,
		FromName:    object.FromName,
		Encryption:  object.Encryption,
		Enabled:     object.Enabled,
		IsDefault:   object.IsDefault,
		Description: object.Description,
		CreatedBy:   object.CreatedBy,
	}
}

// sendEmail 按配置的加密方式发送测试邮件，仅依赖标准库 net/smtp / crypto/tls。
// encryption 取值约定：ssl/tls 隐式 TLS（465）；starttls 先明文握手再升级 TLS；
// none 或空值走明文（587/25）。
func sendEmail(cfg *model.EmailConfig, password, to string) error {
	addr := net.JoinHostPort(cfg.SmtpHost, strconv.Itoa(cfg.SmtpPort))
	msg := buildTestMessage(cfg, to)

	switch strings.ToLower(cfg.Encryption) {
	case "ssl", "tls":
		return sendImplicitTLS(cfg, password, to, addr, msg)
	case "starttls", "start_tls":
		return sendStartTLS(cfg, password, to, addr, msg)
	default:
		return sendPlain(cfg, password, to, addr, msg)
	}
}

// buildTestMessage 构造测试邮件内容（主题走 RFC 2047 编码避免中文乱码）。
func buildTestMessage(cfg *model.EmailConfig, to string) []byte {
	from := cfg.FromEmail
	if cfg.FromName != "" {
		from = fmt.Sprintf("%s <%s>", cfg.FromName, cfg.FromEmail)
	}
	subject := "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte("Pixiu 邮件配置测试")) + "?="
	body := "Pixiu 平台邮件服务配置测试：\r\n\r\n如果收到本邮件，说明 SMTP 配置正确可用。"
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from, to, subject, body)
	return []byte(msg)
}

func sendPlain(cfg *model.EmailConfig, password, to, addr string, msg []byte) error {
	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, password, cfg.SmtpHost)
	}
	return smtp.SendMail(addr, auth, cfg.FromEmail, []string{to}, msg)
}

func sendImplicitTLS(cfg *model.EmailConfig, password, to, addr string, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: cfg.SmtpHost})
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, cfg.SmtpHost)
	if err != nil {
		return err
	}
	defer client.Close()

	if cfg.Username != "" {
		if err = client.Auth(smtp.PlainAuth("", cfg.Username, password, cfg.SmtpHost)); err != nil {
			return err
		}
	}
	return deliverMail(client, cfg.FromEmail, to, msg)
}

func sendStartTLS(cfg *model.EmailConfig, password, to, addr string, msg []byte) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, cfg.SmtpHost)
	if err != nil {
		return err
	}
	defer client.Close()

	if err = client.StartTLS(&tls.Config{ServerName: cfg.SmtpHost}); err != nil {
		return err
	}
	if cfg.Username != "" {
		if err = client.Auth(smtp.PlainAuth("", cfg.Username, password, cfg.SmtpHost)); err != nil {
			return err
		}
	}
	return deliverMail(client, cfg.FromEmail, to, msg)
}

// deliverMail 在已建立（并可能已升级 TLS）的 smtp.Client 上投递邮件。
func deliverMail(client *smtp.Client, from, to string, msg []byte) error {
	if err := client.Mail(from); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err = w.Write(msg); err != nil {
		return err
	}
	if err = w.Close(); err != nil {
		return err
	}
	return client.Quit()
}
