package core

import (
	"github.com/casbin/casbin/v2"

	"github.com/caoyingjunz/gopixiu/cmd/app/config"
	"github.com/caoyingjunz/gopixiu/pkg/db"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	user2 "github.com/caoyingjunz/gopixiu/pkg/db/user"
)

type CasbinInterface interface {
	user2.CasbinInterface
	GetEnforce() *casbin.Enforcer
}
type CasbinGetter interface {
	Casbin() CasbinInterface
}

type casbinStruct struct {
	ComponentConfig config.Config
	app             *pixiu
	factory         db.ShareDaoFactory
}

func newCasbin(c *pixiu) CasbinInterface {
	return &casbinStruct{
		c.cfg,
		c,
		c.factory,
	}
}

//GetEnforce 获取全局enforcer
func (c *casbinStruct) GetEnforce() *casbin.Enforcer {
	return c.factory.Casbin().GetEnforce()
}

// CasbinAddRoleForUser 添加用户角色权限
func (c *casbinStruct) CasbinAddRoleForUser(userid int64, roleIds []int64) (err error) {
	return c.factory.Casbin().CasbinAddRoleForUser(userid, roleIds)
}

// CasbinSetRolePermission 设置角色权限
func (c *casbinStruct) CasbinSetRolePermission(roleId int64, menus *[]model.Menu) (bool, error) {
	ok, err := c.factory.Casbin().CasbinSetRolePermission(roleId, menus)
	return ok, err
}

// CasbinDeleteRole 删除角色
func (c *casbinStruct) CasbinDeleteRole(roleIds []int64) error {
	return c.factory.Casbin().CasbinDeleteRole(roleIds)
}
