package sys

import (
	"context"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	"gorm.io/gorm"
)

// MenuInterface 角色操作接口
type MenuInterface interface {
	Create(context.Context, *model.Menu) (*model.Menu, error)
	Update(context.Context, *model.Menu) error
	Delete(context.Context, int64) error
	Get(context.Context, int64) (*model.Menu, error)
	List(context.Context) ([]model.Menu, error)

	GetByRoleID(context.Context, uint64) (*model.Menu, error)
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

func (m *menu) Update(c context.Context, menu *model.Menu) error {
	return m.db.Updates(*menu).Error
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

func (m *menu) List(c context.Context) (menus []model.Menu, err error) {
	if tx := m.db.Find(&menus); tx.Error != nil {
		return nil, tx.Error
	}
	return
}

/*

db.Table("go_service_info").Select("go_service_info.serviceId as service_id, go_service_info.serviceName as service_name,
go_system_info.systemId as system_id, go_system_info.systemName as system_name").
Joins("left join go_system_info on go_service_info.systemId = go_system_info.systemId
where go_service_info.serviceId <> ? and go_system_info.systemId = ?", "xxx", "xxx").Scan(&results)

*/

//GetByRoleID 根据角色ID查询该角色下所有菜单
func (m *menu) GetByRoleID(c context.Context, roleID uint64) (menu *model.Menu, err error) {
	m.db.Table("menu").Select(" menu.id, menu.parent_id,menu.name, menu.url, menu.icon,menu.code,menu.method").
		Joins("left join role_menu on menu.id = role_menu.menu_id ").Scan(&menu)
	//if err := m.db.Where("name = ?", roleID).First(&menu).Error; err != nil {
	//	return nil, err
	//}
	return
}
