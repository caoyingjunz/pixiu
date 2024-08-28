/*
Copyright 2021 The Pixiu Authors.

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

package errors

import (
	"errors"

	"gorm.io/gorm"

	"github.com/go-sql-driver/mysql"
)

var (
	ErrRecordNotFound     = gorm.ErrRecordNotFound
	ErrRecordNotUpdate    = errors.New("record not updated")
	ErrBusySystem         = errors.New("系统繁忙，请稍后再试")
	ErrReqParams          = errors.New("请求参数错误")
	ErrCloudNotRegister   = errors.New("cloud 集群未注册")
	ErrUserNotFound       = errors.New("用户不存在")
	ErrNotAcceptable      = errors.New("有任务正在执行，请稍后再试")
	ErrClusterNotFound    = errors.New("集群不存在")
	ErrUserPassword       = errors.New("密码错误")
	ErrInternal           = errors.New("服务器内部错误")
	ErrTenantNotFound     = errors.New("租户不存在")
	ErrDuplicatedPassword = errors.New("新密码与旧密码相同")
	ErrAuditNotFound      = errors.New("审计记录不存在")

	ErrContainerNotFound = errors.New("容器不存在")

	ParamsError         = errors.New("参数错误")
	OperateFailed       = errors.New("操作失败")
	NoPermission        = errors.New("无权限")
	InnerError          = errors.New("内部错误")
	NoUserIdError       = errors.New("请登录")
	UserExistError      = errors.New("用户已存在")
	RoleExistError      = errors.New("角色已存在")
	RoleNotExistError   = errors.New("角色不存在")
	PolicyExistError    = errors.New("策略已存在")
	PolicyNotExistError = errors.New("策略不存在")
	TenantExistError    = errors.New("租户已存在")
	ErrAuditExists      = errors.New("审计记录已存在")
)

func IsRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

func IsNotUpdated(err error) bool {
	return errors.Is(err, ErrRecordNotUpdate)
}

func IsUniqueConstraintError(err error) bool {
	mysqlErr, ok := err.(*mysql.MySQLError)
	if !ok {
		return false
	}

	// 数据库的 1062 错误码为固定的主键冲突号
	return mysqlErr.Number == 1062
}
