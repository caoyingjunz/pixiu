package model

import "github.com/caoyingjunz/pixiu/pkg/db/model/pixiu"

type Audit struct {
	pixiu.Model
	Ip           string `gorm:"type:varchar(128)" json:"ip"`
	Action       string `gorm:"type:varchar(255)" json:"action"`        // 操作动作
	Operator     string `gorm:"type:varchar(255)" json:"operator"`      // 操作人
	Path         string `gorm:"type:varchar(255)" json:"path"`          //操作路径
	ResourceType string `gorm:"type:varchar(128)" json:"resource_type"` //操作资源
	Status       int    `gorm:"type:tinyint" json:"status"`             // 操作状态
}

func (a *Audit) TableName() string {
	return "audit"
}
