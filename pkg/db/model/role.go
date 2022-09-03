package model

import (
	"github.com/caoyingjunz/gopixiu/pkg/db/gopixiu"
	"gorm.io/gorm"
	"time"
)

// Role 角色
type Role struct {
	gopixiu.Model
	Memo     string `gorm:"column:memo;size:64;" json:"memo" form:"memo"`                 // 备注
	Name     string `gorm:"column:name;size:32;not null;" json:"name" form:"name"`        // 名称
	Sequence int    `gorm:"column:sequence;not null;" json:"sequence" form:"sequence"`    // 排序值
	ParentID uint64 `gorm:"column:parent_id;not null;" json:"parent_id" form:"parent_id"` // 父级ID
}

// TableName 自定义表名
func (r *Role) TableName() string {
	return "role"
}

// BeforeCreate 创建前操作
func (r *Role) BeforeCreate(*gorm.DB) error {
	r.GmtCreate = time.Now()
	r.GmtModified = time.Now()
	return nil
}

// BeforeUpdate 更新前操作
func (r *Role) BeforeUpdate(*gorm.DB) error {
	r.GmtModified = time.Now()
	r.ResourceVersion++
	return nil
}
