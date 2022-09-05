package core

import (
	"context"

	"github.com/caoyingjunz/gopixiu/cmd/app/config"
	"github.com/caoyingjunz/gopixiu/pkg/db"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	user2 "github.com/caoyingjunz/gopixiu/pkg/db/user"
	"github.com/caoyingjunz/gopixiu/pkg/log"
)

// RoleInterface 角色操作接口
type RoleInterface interface {
	user2.RoleInterface
}

type RoleGetter interface {
	Role() RoleInterface
}

type role struct {
	ComponentConfig config.Config
	app             *pixiu
	factory         db.ShareDaoFactory
}

func newRole(c *pixiu) RoleInterface {
	return &role{
		c.cfg,
		c,
		c.factory,
	}
}
func (r *role) Create(c context.Context, obj *model.Role) (role *model.Role, err error) {
	if role, err = r.factory.Role().Create(c, obj); err != nil {
		return
	}
	return
}

func (r *role) Update(c context.Context, role *model.Role) error {
	return r.factory.Role().Update(c, role)
}

func (r *role) Delete(c context.Context, rid []int64) error {
	return r.factory.Role().Delete(c, rid)
}

func (r *role) Get(c context.Context, rid int64) (roles *[]model.Role, err error) {
	if roles, err = r.factory.Role().Get(c, rid); err != nil {
		return nil, err
	}
	return
}

func (r *role) List(c context.Context) (roles []model.Role, err error) {
	if roles, err = r.factory.Role().List(c); err != nil {
		return
	}
	return
}
func (r *role) GetMenusByRoleID(c context.Context, rid int64) (*[]model.Menu, error) {
	menus, err := r.factory.Role().GetMenusByRoleID(c, rid)
	if err != nil {
		log.Logger.Errorf(err.Error())
		return menus, err
	}
	return menus, err
}

// SetRole 设置角色菜单权限
func (r *role) SetRole(ctx context.Context, roleId int64, menuIds []int64) error {
	if err := r.factory.Role().SetRole(ctx, roleId, menuIds); err != nil {
		log.Logger.Errorf(err.Error())
		return err
	}
	return nil
}
