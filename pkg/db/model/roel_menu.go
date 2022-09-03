package model

import (
	"github.com/caoyingjunz/gopixiu/pkg/db/gopixiu"
	"gorm.io/gorm"
	"time"
)

// RoleMenu 角色-菜单
type RoleMenu struct {
	gopixiu.Model
	RoleID uint64 `gorm:"column:role_id;unique_index:uk_role_menu_role_id;not null;"` // 角色ID
	MenuID uint64 `gorm:"column:menu_id;unique_index:uk_role_menu_role_id;not null;"` // 菜单ID
}

// TableName 表名
func (m *RoleMenu) TableName() string {
	return "role_menu"
}

// BeforeCreate 添加前
func (m *RoleMenu) BeforeCreate(*gorm.DB) error {
	m.GmtCreate = time.Now()
	m.GmtModified = time.Now()
	return nil
}

// BeforeUpdate 更新前
func (m *RoleMenu) BeforeUpdate(*gorm.DB) error {
	m.GmtModified = time.Now()
	m.ResourceVersion++
	return nil
}
