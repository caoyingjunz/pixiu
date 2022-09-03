package model

import "github.com/caoyingjunz/gopixiu/pkg/db/gopixiu"

type RoleMenuButton struct {
	gopixiu.Model
	RoleID int64
	Type   int
	RefID  int ``
}
