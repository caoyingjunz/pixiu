package model

import (
	"time"

	"gorm.io/gorm"

	"github.com/caoyingjunz/gopixiu/pkg/db/gopixiu"
)

// RoleMenu 角色-菜单
type RoleMenu struct {
	gopixiu.Model
	RoleID int64 `gorm:"column:role_id;unique_index:uk_role_menu_role_id;not null;" json:"role_id"`  // 角色ID
	MenuID int64 `gorm:"column:menu_id;unique_index:uk_role_menu_role_id;not null;" json:"menu_id'"` // 菜单ID
}

// TableName 表名
func (m *RoleMenu) TableName() string {
	return "role_menus"
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
	return nil
}
