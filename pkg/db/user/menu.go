package user

import (
	"context"

	"gorm.io/gorm"

	"github.com/caoyingjunz/gopixiu/pkg/db/model"
)

// MenuInterface 角色操作接口
type MenuInterface interface {
	Create(context.Context, *model.Menu) (*model.Menu, error)
	Update(context.Context, *model.Menu, int64) error
	Delete(context.Context, int64) error
	Get(context.Context, int64) (*model.Menu, error)
	List(context.Context) ([]model.TreeMenus, error)

	GetByRoleID(context.Context, uint64) (*model.Menu, error)
	GetByIds(c context.Context, mIds []int64) (menus *[]model.Menu, err error)
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

func (m *menu) Update(c context.Context, menu *model.Menu, mId int64) error {
	//return m.db.Model(&model.Menu{}).Where("id = ?", mId).Updates(&menu).Error
	//return m.db.Save(&menu).Where("resource_version = ?", menu.ResourceVersion).Error
	var mm model.Menu
	//m.db.Model(&mm).Select("*").Updates(menu)
	return m.db.Model(&mm).Updates(menu).Error
}

func (m *menu) Delete(c context.Context, mid int64) error {
	return m.db.Delete(model.Menu{}, mid).Error
}

func (m *menu) Get(c context.Context, mid int64) (menu *model.Menu, err error) {
	if err := m.db.Where("id = ?", mid).First(&menu).Error; err != nil {
		return nil, err
	}
	return
}

func (m *menu) List(c context.Context) (treeMenusList []model.TreeMenus, err error) {
	var menus []model.Menu
	err = m.db.Find(&menus).Error
	treeMenusList = getTreeMenus(menus, 0)
	return
}

//GetByRoleID 根据角色ID查询该角色下所有菜单
func (m *menu) GetByRoleID(c context.Context, roleID uint64) (menu *model.Menu, err error) {
	m.db.Table("menu").Select(" menu.id, menu.parent_id,menu.name, menu.url, menu.icon,menu.code,menu.method").
		Joins("left join role_menu on menu.id = role_menu.menu_id ").Scan(&menu)

	return
}

func (m *menu) GetByIds(c context.Context, mIds []int64) (menus *[]model.Menu, err error) {
	if err := m.db.Where("id in ?", mIds).Find(&menus).Error; err != nil {
		return nil, err
	}
	return
}

func getTreeMenus(menusList []model.Menu, pid int64) (treeMenusList []model.TreeMenus) {
	for _, v := range menusList {
		if v.ParentID == pid {
			child := getTreeMenus(menusList, v.Id)
			node := model.TreeMenus{Menus: v}
			node.Children = child
			treeMenusList = append(treeMenusList, node)
		}
	}
	return treeMenusList
}
