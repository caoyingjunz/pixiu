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

package cluster

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
	"github.com/caoyingjunz/pixiu/pkg/util/token"
)

// AuthorizeClusterAccess 校验用户是否可访问指定集群（按 id）。
func (c *cluster) AuthorizeClusterAccess(ctx context.Context, user *model.User, clusterId int64) (*model.Cluster, error) {
	if user == nil {
		return nil, errors.ErrUnauthorized
	}
	obj, err := c.factory.Cluster().Get(ctx, clusterId)
	if err != nil {
		klog.Errorf("failed to get cluster(%d): %v", clusterId, err)
		return nil, errors.ErrServerInternal
	}
	if obj == nil {
		return nil, errors.ErrClusterNotFound
	}
	if err = c.ensureClusterAccess(ctx, user, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

// AuthorizeClusterAccessByName 校验用户是否可访问指定集群（按 name）。
func (c *cluster) AuthorizeClusterAccessByName(ctx context.Context, user *model.User, clusterName string) (*model.Cluster, error) {
	if user == nil {
		return nil, errors.ErrUnauthorized
	}
	obj, err := c.factory.Cluster().GetClusterByName(ctx, clusterName)
	if err != nil {
		klog.Errorf("failed to get cluster(%s): %v", clusterName, err)
		return nil, errors.ErrServerInternal
	}
	if obj == nil {
		return nil, errors.ErrClusterNotFound
	}
	if err = c.ensureClusterAccess(ctx, user, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func (c *cluster) ensureClusterAccess(ctx context.Context, user *model.User, obj *model.Cluster) error {
	if user.Role == model.RoleRoot {
		return nil
	}
	if obj.UserId == user.Id {
		return nil
	}
	perms, err := c.factory.Permission().List(ctx, db.WithUser(user.Id), db.WithOwnerCluster(obj.Id))
	if err != nil {
		klog.Errorf("failed to list permissions for user(%d) cluster(%d): %v", user.Id, obj.Id, err)
		return errors.ErrServerInternal
	}
	if len(perms) > 0 {
		return nil
	}
	return errors.ErrForbidden
}

// CreateProxyKubeconfig 签发 Access Token 并生成指向 Pixiu 的标准 kubeconfig。
func (c *cluster) CreateProxyKubeconfig(ctx context.Context, clusterId int64, req *types.CreateProxyKubeconfigRequest) (*types.ProxyKubeconfigResponse, error) {
	if !c.cc.KubeGateway.IsEnabled() {
		return nil, errors.NewError(fmt.Errorf("kube gateway is disabled"), http.StatusServiceUnavailable)
	}
	user, err := httputils.GetUserFromRequest(ctx)
	if err != nil {
		return nil, errors.ErrUnauthorized
	}
	obj, err := c.AuthorizeClusterAccess(ctx, user, clusterId)
	if err != nil {
		return nil, err
	}

	gw := c.cc.KubeGateway
	gw.Normalize()
	expireHours := req.ExpireHours
	if expireHours <= 0 {
		expireHours = gw.DefaultExpireHours
	}
	if expireHours > gw.MaxExpireHours {
		return nil, errors.NewError(fmt.Errorf("expire_hours exceeds max %d", gw.MaxExpireHours), http.StatusBadRequest)
	}

	plaintext, jti, hash, err := token.GenerateKubeAccessToken()
	if err != nil {
		return nil, errors.ErrServerInternal
	}
	expireAt := time.Now().Add(time.Duration(expireHours) * time.Hour)
	record := &model.ClusterAccessToken{
		JTI:         jti,
		UserId:      user.Id,
		ClusterId:   obj.Id,
		ClusterName: obj.Name,
		Name:        strings.TrimSpace(req.Name),
		TokenHash:   hash,
		ExpiresAt:   &expireAt,
	}
	if _, err = c.factory.ClusterAccessToken().Create(ctx, record); err != nil {
		klog.Errorf("failed to create cluster access token: %v", err)
		return nil, errors.ErrServerInternal
	}

	server := c.buildGatewayServer(ctx, obj.Name)
	kubeconfigYAML, err := buildProxyKubeconfigYAML(obj, user.Id, server, plaintext, gw.InsecureSkipTLSVerify)
	if err != nil {
		return nil, errors.ErrServerInternal
	}

	return &types.ProxyKubeconfigResponse{
		ClusterId:          obj.Id,
		ClusterName:        obj.Name,
		AliasName:          obj.AliasName,
		JTI:                jti,
		ExpireAt:           expireAt.Format(time.RFC3339),
		Server:             server,
		Token:              plaintext,
		KubeConfig:         kubeconfigYAML,
		KubeConfigEncoding: "yaml",
	}, nil
}

// RevokeAccessToken 吊销指定 jti 的访问令牌（仅 token 所有者）。
func (c *cluster) RevokeAccessToken(ctx context.Context, clusterId int64, jti string) error {
	user, err := httputils.GetUserFromRequest(ctx)
	if err != nil {
		return errors.ErrUnauthorized
	}
	if _, err = c.AuthorizeClusterAccess(ctx, user, clusterId); err != nil {
		return err
	}
	jti = strings.TrimSpace(jti)
	if jti == "" {
		return errors.ErrInvalidRequest
	}
	if err = c.factory.ClusterAccessToken().RevokeByJTI(ctx, jti, user.Id); err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return errors.NewError(fmt.Errorf("access token not found"), http.StatusNotFound)
		}
		klog.Errorf("failed to revoke access token(%s): %v", jti, err)
		return errors.ErrServerInternal
	}
	return nil
}

// ValidateKubeAccessToken 校验网关 Bearer token，返回用户与 token 记录。
func (c *cluster) ValidateKubeAccessToken(ctx context.Context, plaintext string) (*model.User, *model.ClusterAccessToken, error) {
	plaintext = strings.TrimSpace(plaintext)
	if !token.IsKubeAccessToken(plaintext) {
		return nil, nil, errors.ErrUnauthorized
	}
	hash := token.HashKubeAccessToken(plaintext)
	rec, err := c.factory.ClusterAccessToken().GetByTokenHash(ctx, hash)
	if err != nil {
		klog.Errorf("failed to get access token by hash: %v", err)
		return nil, nil, errors.ErrServerInternal
	}
	if rec == nil || !rec.IsActive() {
		return nil, nil, errors.ErrUnauthorized
	}
	user, err := c.factory.User().Get(ctx, rec.UserId)
	if err != nil || user == nil {
		return nil, nil, errors.ErrUnauthorized
	}
	if user.Status == model.UserStatusForbidden {
		return nil, nil, errors.ErrForbidden
	}
	_ = c.factory.ClusterAccessToken().TouchLastUsed(ctx, rec.Id)
	return user, rec, nil
}

func (c *cluster) buildGatewayServer(ctx context.Context, clusterName string) string {
	base := strings.TrimRight(strings.TrimSpace(c.cc.Default.PublicURL), "/")
	if base == "" {
		base = inferPublicURL(ctx)
	}
	if base == "" {
		base = "https://localhost"
	}
	return fmt.Sprintf("%s/k8s/%s", base, clusterName)
}

func inferPublicURL(ctx context.Context) string {
	gc, ok := ctx.(*gin.Context)
	if !ok || gc == nil || gc.Request == nil {
		return ""
	}
	proto := gc.GetHeader("X-Forwarded-Proto")
	if proto == "" {
		if gc.Request.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	host := gc.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = gc.Request.Host
	}
	if host == "" {
		return ""
	}
	return proto + "://" + host
}

func buildProxyKubeconfigYAML(obj *model.Cluster, userId int64, server, accessToken string, insecureSkipTLS bool) (string, error) {
	ctxName := obj.AliasName
	if ctxName == "" {
		ctxName = obj.Name
	}
	userName := fmt.Sprintf("pixiu-%d", userId)

	cfg := clientcmdapi.NewConfig()
	clusterCfg := &clientcmdapi.Cluster{Server: server}
	if insecureSkipTLS {
		clusterCfg.InsecureSkipTLSVerify = true
	}
	cfg.Clusters[ctxName] = clusterCfg
	cfg.AuthInfos[userName] = &clientcmdapi.AuthInfo{Token: accessToken}
	cfg.Contexts[ctxName] = &clientcmdapi.Context{
		Cluster:  ctxName,
		AuthInfo: userName,
	}
	cfg.CurrentContext = ctxName

	b, err := clientcmd.Write(*cfg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
