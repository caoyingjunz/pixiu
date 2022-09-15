package model

import (
	"time"

	"gorm.io/gorm"

	"github.com/caoyingjunz/gopixiu/pkg/db/gopixiu"
)

type UserRole struct {
	gopixiu.Model
	UserID int64 `gorm:"column:user_id;unique_index:uk_user_role_user_id;not null;" json:"user_id"` // 管理员ID
	RoleID int64 `gorm:"column:role_id;unique_index:uk_user_role_user_id;not null;" json:"role_id"` // 角色ID
}

// TableName 自定义表名
func (u *UserRole) TableName() string {
	return "user_roles"
}

// BeforeCreate 添加前
func (u *UserRole) BeforeCreate(*gorm.DB) error {
	u.GmtCreate = time.Now()
	u.GmtModified = time.Now()
	return nil
}

// BeforeUpdate 更新前
func (u *UserRole) BeforeUpdate(*gorm.DB) error {
	u.GmtModified = time.Now()
	return nil
}
