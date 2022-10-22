package user

import (
	"context"
	"errors"

	"github.com/fatih/structs"
	"gorm.io/gorm"

	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	"github.com/caoyingjunz/gopixiu/pkg/log"
)

// MenuInterface 菜单操作接口
type MenuInterface interface {
	Create(context.Context, *model.Menu) (*model.Menu, error)
	Update(context.Context, *types.UpdateMenusReq, int64) error
	Delete(context.Context, int64) error
	Get(context.Context, int64) (*model.Menu, error)
	List(context.Context) ([]model.Menu, error)

	GetByIds(context.Context, []int64) (*[]model.Menu, error)
	GetMenuByMenuNameUrl(context.Context, string, string) (*model.Menu, error)
	UpdateStatus(c context.Context, menuId, status int64) error
}

type menu struct {
	db *gorm.DB
}

func NewMenu(db *gorm.DB) *menu {
	return &menu{db}
}

func (m *menu) Create(c context.Context, obj *model.Menu) (*model.Menu, error) {
	if err := m.db.Create(obj).Error; err != nil {
		return nil, err
	}
	return obj, nil
}

func (m *menu) Update(c context.Context, obj *types.UpdateMenusReq, mId int64) error {
	resourceVersion := obj.ResourceVersion
	obj.ResourceVersion++
	objMap := structs.Map(obj)
	tx := m.db.Model(&model.Menu{}).Where("id = ? and resource_version = ? ", mId, resourceVersion).Updates(objMap)
	if tx.RowsAffected == 0 {
		return errors.New("update failed")
	}
	return tx.Error
}

func (m *menu) Delete(c context.Context, mId int64) error {
	tx := m.db.Begin()
	defer func() {
		if err := recover(); err != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Error; err != nil {
		tx.Rollback()
		log.Logger.Errorf(err.Error())
		return err
	}

	// 清除role_menus
	if err := tx.Where("menu_id = ?", mId).Delete(&model.RoleMenu{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 清除menus
	if err := tx.Where("id =  ?", mId).
		Or("parent_id = ?", mId).
		Delete(&model.Menu{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

func (m *menu) Get(c context.Context, mId int64) (menu *model.Menu, err error) {
	if err = m.db.Where("id = ?", mId).First(&menu).Error; err != nil {
		return nil, err
	}
	return
}

func (m *menu) List(c context.Context) (treeMenusList []model.Menu, err error) {
	var menus []model.Menu
	err = m.db.Order("sequence DESC").Find(&menus).Error
	treeMenusList = getTreeMenus(menus, 0)
	return
}

func (m *menu) GetByIds(c context.Context, mIds []int64) (menus *[]model.Menu, err error) {
	if err = m.db.Where("id in ?", mIds).Find(&menus).Error; err != nil {
		return nil, err
	}
	return
}

func (m *menu) GetMenuByMenuNameUrl(c context.Context, url, method string) (menu *model.Menu, err error) {
	err = m.db.Where("url = ? and method = ?", url, method).First(&menu).Error
	return
}

func (m *menu) UpdateStatus(c context.Context, menuId, status int64) error {
	return m.db.Model(&model.Menu{}).Where("id = ?", menuId).Update("status", status).Error
}

func getTreeMenus(menusList []model.Menu, pid int64) (treeMenusList []model.Menu) {
	for _, node := range menusList {
		if node.ParentID == pid {
			child := getTreeMenus(menusList, node.Id)
			node.Children = child
			treeMenusList = append(treeMenusList, node)
		}
	}
	return treeMenusList
}
