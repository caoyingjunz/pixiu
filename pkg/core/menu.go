package core

import (
	"context"

	"github.com/caoyingjunz/gopixiu/cmd/app/config"
	"github.com/caoyingjunz/gopixiu/pkg/db"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	menus "github.com/caoyingjunz/gopixiu/pkg/db/user"
	"github.com/caoyingjunz/gopixiu/pkg/log"
)

// MenuInterface 菜单操作接口
type MenuInterface interface {
	menus.MenuInterface
}

type MenuGetter interface {
	Menu() MenuInterface
}

type menu struct {
	ComponentConfig config.Config
	app             *pixiu
	factory         db.ShareDaoFactory
}

func newMenu(c *pixiu) *menu {
	return &menu{
		c.cfg,
		c,
		c.factory,
	}
}

func (m *menu) Create(c context.Context, obj *model.Menu) (menu *model.Menu, err error) {
	if menu, err = m.factory.Menu().Create(c, obj); err != nil {
		log.Logger.Error(err)
		return
	}
	return
}

func (m *menu) Update(c context.Context, menu *model.Menu, mId int64) error {
	err := m.factory.Menu().Update(c, menu, mId)
	if err != nil {
		log.Logger.Error(err)
		return err
	}
	return nil
}

func (m *menu) Delete(c context.Context, mId int64) error {
	menuInfo, err := m.factory.Menu().Get(c, mId)
	if err != nil {
		log.Logger.Error(err)
		return err
	}

	err = m.factory.Menu().Delete(c, mId)
	if err != nil {
		log.Logger.Error(err)
		return err
	}

	//cabin 删除role对应的权限
	go m.factory.Authentication().DeleteRolePermission(menuInfo.URL, menuInfo.Method)
	return nil
}

func (m *menu) Get(c context.Context, mId int64) (menu *model.Menu, err error) {
	if menu, err = m.factory.Menu().Get(c, mId); err != nil {
		log.Logger.Error(err)
		return nil, err
	}
	return
}

func (m *menu) List(c context.Context) (menus []model.Menu, err error) {
	if menus, err = m.factory.Menu().List(c); err != nil {
		log.Logger.Error(err)
		return
	}
	return
}

func (m *menu) GetByIds(c context.Context, mIds []int64) (menus *[]model.Menu, err error) {
	menus, err = m.factory.Menu().GetByIds(c, mIds)
	if err != nil {
		log.Logger.Error(err)
		return
	}
	return
}
