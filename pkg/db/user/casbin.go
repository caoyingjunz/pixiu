package user

import (
	"strconv"

	"github.com/casbin/casbin/v2"
	"gorm.io/gorm"

	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	"github.com/caoyingjunz/gopixiu/pkg/log"
)

type CasbinInterface interface {
	GetEnforce() *casbin.Enforcer
	CasbinAddRoleForUser(userid int64, roleIds []int64) (err error)
	CasbinSetRolePermission(roleId int64, menus *[]model.Menu) (bool, error)
	CasbinDeleteRole(roleIds []int64) error
}

type casbinStruct struct {
	db       *gorm.DB
	enforcer *casbin.Enforcer
}

func NewCasbin(db *gorm.DB, enforcer *casbin.Enforcer) *casbinStruct {
	return &casbinStruct{db, enforcer}
}

func (c *casbinStruct) GetEnforce() *casbin.Enforcer {
	return c.enforcer
}

// CasbinAddRoleForUser 分配用户角色
func (c *casbinStruct) CasbinAddRoleForUser(userid int64, roleIds []int64) (err error) {
	uidStr := strconv.FormatInt(userid, 10)
	c.enforcer.DeleteRolesForUser(uidStr)
	for _, roleId := range roleIds {
		c.enforcer.AddRoleForUser(uidStr, strconv.FormatInt(roleId, 10))
	}
	return
}

// CasbinSetRolePermission 设置角色权限
func (c *casbinStruct) CasbinSetRolePermission(roleId int64, menus *[]model.Menu) (bool, error) {
	c.enforcer.DeletePermissionsForUser(strconv.FormatInt(roleId, 10))
	c.setRolePermission(roleId, menus)
	return false, nil
}

// 设置角色权限
func (c *casbinStruct) setRolePermission(roleId int64, menus *[]model.Menu) (bool, error) {
	for _, menu := range *menus {
		if menu.MenuType == 2 {
			ok, err := c.enforcer.AddPermissionForUser(strconv.FormatInt(roleId, 10), menu.URL, menu.Method)
			if !ok || err != nil {
				return ok, err
			}
		}
	}
	return false, nil
}

// CasbinDeleteRole 删除角色
func (c *casbinStruct) CasbinDeleteRole(roleIds []int64) error {
	for _, rid := range roleIds {
		_, err := c.enforcer.DeletePermissionsForUser(strconv.FormatInt(rid, 10))
		if err != nil {
			log.Logger.Errorf(err.Error())
			return err
		}
		_, err = c.enforcer.DeleteRole(strconv.FormatInt(rid, 10))
		if err != nil {
			log.Logger.Errorf(err.Error())
			return err
		}
	}
	return nil
}
