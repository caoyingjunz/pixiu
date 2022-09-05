package model

import (
	"time"

	"gorm.io/gorm"

	"github.com/caoyingjunz/gopixiu/pkg/db/gopixiu"
)

//Menu 菜单
type Menu struct {
	gopixiu.Model
	Status   uint8  `gorm:"column:status;type:tinyint(1);not null;" json:"status" form:"status"`                       // 状态(1:启用 2:不启用)
	Memo     string `gorm:"column:memo;size:64;" json:"memo,omitempty" form:"memo"`                                    // 备注
	ParentID uint64 `gorm:"column:parent_id;not null;" json:"parent_id,omitempty" form:"parent_id"`                    // 父级ID
	URL      string `gorm:"column:url;size:72;" json:"url,omitempty" form:"url"`                                       // 菜单URL
	Name     string `gorm:"column:name;size:32;not null;" json:"name" form:"name"`                                     // 菜单名称
	Sequence int    `gorm:"column:sequence;not null;" json:"sequence" form:"sequence"`                                 // 排序值
	MenuType uint8  `gorm:"column:menu_type;type:tinyint(1);not null;" json:"menu_type" form:"menu_type"`              // 菜单类型 1 左侧菜单(),2 菜单(可执行操作),3 操作
	Code     string `gorm:"column:code;size:32;not null;unique_index:uk_menu_code;" json:"code,omitempty" form:"code"` // 菜单代码
	Icon     string `gorm:"column:icon;size:32;" json:"icon,omitempty" form:"icon"`                                    // icon
	Method   string `gorm:"column:method;size:32;not null;" json:"method,omitempty" form:"method"`                     // 操作类型 none/GET/POST/PUT/DELETE
}

// TableName 表名
func (m *Menu) TableName() string {
	return "menu"
}

// BeforeCreate 添加前
func (m *Menu) BeforeCreate(*gorm.DB) error {
	m.GmtCreate = time.Now()
	m.GmtModified = time.Now()
	m.Status = 1
	return nil
}

// BeforeUpdate 更新前
func (m *Menu) BeforeUpdate(*gorm.DB) error {
	m.GmtModified = time.Now()
	m.ResourceVersion++
	return nil
}
