package core

import (
	"context"
	"github.com/caoyingjunz/gopixiu/cmd/app/config"
	"github.com/caoyingjunz/gopixiu/pkg/db"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	"github.com/caoyingjunz/gopixiu/pkg/db/sys"
)

// MenuInterface 菜单操作接口
type MenuInterface interface {
	sys.MenuInterface
}

type MenuGetter interface {
	Menu() MenuInterface
}

type menu struct {
	ComponentConfig config.Config
	app             *pixiu
	factory         db.ShareDaoFactory
}

func newMenu(c *pixiu) sys.MenuInterface {
	return &menu{
		c.cfg,
		c,
		c.factory,
	}
}
func (m *menu) Create(c context.Context, obj *model.Menu) (menu *model.Menu, err error) {
	if menu, err = m.factory.Menu().Create(c, obj); err != nil {
		return
	}
	return
}

func (m *menu) Update(c context.Context, menu *model.Menu) error {
	return m.factory.Menu().Update(c, menu)
}

func (m *menu) Delete(c context.Context, rid int64) error {
	return m.factory.Menu().Delete(c, rid)
}

func (m *menu) Get(c context.Context, rid int64) (menu *model.Menu, err error) {
	if menu, err = m.factory.Menu().Get(c, rid); err != nil {
		return nil, err
	}
	return
}

func (m *menu) List(c context.Context) (menus []model.Menu, err error) {
	if menus, err = m.factory.Menu().List(c); err != nil {
		return
	}
	return
}

func (m *menu) GetByRoleID(c context.Context, roleID uint64) (menu *model.Menu, err error) {
	if menu, err = m.factory.Menu().GetByRoleID(c, roleID); err != nil {
		return nil, err
	}
	return
}
