package model

import "github.com/caoyingjunz/pixiu/pkg/db/model/pixiu"

const (
	DeletedAudit = "删除"
	CreatedAudit = "新增"
	UpdatedAudit = "更改"
)

type Audit struct {
	pixiu.Model
	Ip        string `gorm:"type:varchar(128)" json:"ip"`
	Action    string `gorm:"type:varchar(255)" json:"action"`   // 操作动作
	Content   string `gorm:"type:text" json:"content"`          // 操作内容
	Operator  string `gorm:"type:varchar(255)" json:"operator"` // 操作人
	IsDeleted bool   `gorm:"type:tinyint"`
}

func (a *Audit) TableName() string {
	return "audit"
}
