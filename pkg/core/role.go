package core

import (
	"context"

	"github.com/caoyingjunz/gopixiu/cmd/app/config"
	"github.com/caoyingjunz/gopixiu/pkg/db"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	roles "github.com/caoyingjunz/gopixiu/pkg/db/user"
	"github.com/caoyingjunz/gopixiu/pkg/log"
)

// RoleInterface 角色操作接口
type RoleInterface interface {
	roles.RoleInterface
}

type RoleGetter interface {
	Role() RoleInterface
}

type role struct {
	ComponentConfig config.Config
	app             *pixiu
	factory         db.ShareDaoFactory
}

func newRole(c *pixiu) *role {
	return &role{
		c.cfg,
		c,
		c.factory,
	}
}
func (r *role) Create(c context.Context, obj *model.Role) (role *model.Role, err error) {
	if role, err = r.factory.Role().Create(c, obj); err != nil {
		log.Logger.Error(err)
		return
	}
	return
}

func (r *role) Update(c context.Context, role *model.Role, rid int64) error {
	if err := r.factory.Role().Update(c, role, rid); err != nil {
		log.Logger.Error(err)
		return err
	}
	return nil
}

func (r *role) Delete(c context.Context, rid []int64) error {
	err := r.factory.Role().Delete(c, rid)
	if err != nil {
		log.Logger.Error(err)
		return err
	}
	go r.factory.Authentication().DeleteRole(rid)
	return nil
}

func (r *role) Get(c context.Context, rid int64) (roles *[]model.Role, err error) {
	if roles, err = r.factory.Role().Get(c, rid); err != nil {
		log.Logger.Error(err)
		return
	}
	return
}

func (r *role) List(c context.Context) (roles *[]model.Role, err error) {
	if roles, err = r.factory.Role().List(c); err != nil {
		log.Logger.Error(err)
		return
	}
	return
}
func (r *role) GetMenusByRoleID(c context.Context, rid int64) (*[]model.Menu, error) {
	menus, err := r.factory.Role().GetMenusByRoleID(c, rid)
	if err != nil {
		log.Logger.Error(err)
		return menus, err
	}
	return menus, err
}

// SetRole 设置角色菜单权限
func (r *role) SetRole(ctx context.Context, roleId int64, menuIds []int64) error {
	if err := r.factory.Role().SetRole(ctx, roleId, menuIds); err != nil {
		log.Logger.Error(err)
		return err
	}
	menus, err := r.factory.Menu().GetByIds(ctx, menuIds)
	if err != nil {
		log.Logger.Error(err)
		return err
	}
	go r.factory.Authentication().SetRolePermission(roleId, menus)
	return nil
}

func (r *role) GetRolesByMenuID(ctx context.Context, menuId int64) (roleIds *[]int64, err error) {
	roleIds, err = r.factory.Role().GetRolesByMenuID(ctx, menuId)
	if err != nil {
		log.Logger.Error(err)
		return
	}
	return
}
