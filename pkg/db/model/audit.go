package model

import "github.com/caoyingjunz/pixiu/pkg/db/model/pixiu"

const (
	//操作动作
	DeletedAudit = "删除"
	CreatedAudit = "创建"
	UpdatedAudit = "更改"
	StartedAudit = "启动"
	StoppedAudit = "停止"

	//操作详细内容
	UserCreate          = "创建用户"
	UserDelete          = "删除用户"
	UserUpdate          = "更新用户"
	UserPasswordUpdate  = "更新用户密码"
	ClusterCreate       = "创建集群"
	ClusterDelete       = "删除集群"
	ClusterUpdate       = "更新集群"
	ClusterProtectStart = "开启集群保护"
	ClusterProtectStop  = "关闭集群保护"
	PlanCreate          = "创建部署计划"
	PlanDelete          = "删除部署计划"
	PlanUpdate          = "更新部署计划"
	PlanStart           = "启动部署计划"
	PlanStop            = "停止部署计划"
	PlanNodeCreate      = "创建部署计划节点"
	PlanNodeDelete      = "删除部署计划节点"
	PlanNodeUpdate      = "更新部署计划节点"
	PlanConfigCreate    = "创建部署计划配置"
	PlanConfigDelete    = "删除部署计划配置"
	PlanConfigUpdate    = "更新部署计划配置"
	PlanRunTask         = "运行部署计划任务"
	TenantCreate        = "创建租户"
	TenantDelete        = "删除租户"
	TenantUpdate        = "更新租户"
)

type Audit struct {
	pixiu.Model
	Ip       string `gorm:"type:varchar(128)" json:"ip"`
	Action   string `gorm:"type:varchar(255)" json:"action"`   // 操作动作
	Content  string `gorm:"type:text" json:"content"`          // 操作内容
	Operator string `gorm:"type:varchar(255)" json:"operator"` // 操作人
}

func (a *Audit) TableName() string {
	return "audit"
}
