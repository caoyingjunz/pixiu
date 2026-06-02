package model

import (
	"github.com/caoyingjunz/pixiu/pkg/db/model/pixiu"
)

func init() {
	register(&Permission{})
}

type Permission struct {
	pixiu.Model

	UserId int64 `json:"user_id"`

	Name              string `json:"name" binding:"required"`
	ExpirationSeconds int64  `json:"expiration_seconds"`     // 默认 1 年
	PType             int    `json:"p_type"`                 // 0 只读，1 自定义，自定义的时候需要传入rule  2 管理员
	Rules             string `gorm:"type:text" json:"rules"` // 如果 p_type是 1的时候，使用 Rules
	SAName            string `json:"sa_name"`
	SANamespace       string `json:"sa_namespace"`
	TargetNamespaces  string `gorm:"type:text" json:"target_namespaces"`

	// k8s kubeConfig base64 字段
	KubeConfig string `json:"kube_config"`

	// 集群用途描述，可以为空
	Description string `gorm:"type:text" json:"description"`
}

func (*Permission) TableName() string {
	return "permissions"
}
