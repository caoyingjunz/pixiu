package user

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	"github.com/caoyingjunz/gopixiu/pkg/log"
)

// RoleInterface 角色操作接口
type RoleInterface interface {
	Create(context.Context, *model.Role) (*model.Role, error)
	Update(context.Context, *model.Role, int64) error
	Delete(context.Context, []int64) error
	Get(context.Context, int64) (*[]model.Role, error)
	List(context.Context) ([]model.Role, error)
	GetMenusByRoleID(c context.Context, rid int64) (*[]model.Menu, error)
	SetRole(ctx context.Context, roleId int64, menuIds []int64) error
}

type role struct {
	db *gorm.DB
}

func NewRole(db *gorm.DB) *role {
	return &role{db}
}

func (r *role) Create(c context.Context, obj *model.Role) (*model.Role, error) {

	if err := r.db.Create(obj).Error; err != nil {
		log.Logger.Errorf(err.Error())
		return nil, err
	}
	return obj, nil
}

func (r *role) Update(c context.Context, role *model.Role, rid int64) error {
	resourceVersion := role.ResourceVersion
	role.ResourceVersion++
	tx := r.db.Where("id = ? and resource_version = ? ", rid, resourceVersion).Updates(role)
	log.Logger.Info(tx.RowsAffected)
	if tx.RowsAffected == 0 {
		return errors.New("update failed")
	}
	return tx.Error
}

func (r *role) Delete(c context.Context, rids []int64) error {
	tx := r.db.Begin()
	defer func() {
		if err := recover(); err != nil {
			tx.Rollback()
			log.Logger.Errorf(err.(error).Error())

		}
	}()
	if err := tx.Error; err != nil {
		tx.Rollback()
		log.Logger.Errorf(err.Error())
		return err
	}
	// 删除角色
	if err := tx.Where("id in ?", rids).Delete(&model.Role{}).Error; err != nil {
		tx.Rollback()
		log.Logger.Errorf(err.Error())
		return err
	}
	//删除角色相关的菜单
	if err := tx.Where("role_id in ?", rids).Delete(&model.RoleMenu{}).Error; err != nil {
		tx.Rollback()
		log.Logger.Errorf(err.Error())
		return err
	}
	return tx.Commit().Error
}

func (r *role) Get(c context.Context, rid int64) (roles *[]model.Role, err error) {
	if err := r.db.Where("id = ?", rid).Find(&roles).Error; err != nil {
		return nil, err
	}
	return
}

func (r *role) List(c context.Context) (roles []model.Role, err error) {
	if tx := r.db.Find(&roles); tx.Error != nil {
		return nil, tx.Error
	}
	return
}
func (r *role) GetMenusByRoleID(c context.Context, rid int64) (*[]model.Menu, error) {
	var menus []model.Menu
	err := r.db.Table("menu").Select(" menu.id, menu.parent_id,menu.name, menu.url, menu.icon,menu.sequence,menu.code,menu.method").
		Joins("left join role_menu on menu.id = role_menu.menu_id", rid).
		Where("role_menu.role_id = ?", rid).
		Order("parent_id ASC").
		Order("sequence ASC").
		Scan(&menus).Error
	if err != nil {
		log.Logger.Errorf(err.Error())
		return nil, err
	}
	return &menus, nil
}

// SetRole 设置角色菜单权限
func (r *role) SetRole(ctx context.Context, roleId int64, menuIds []int64) error {
	tx := r.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	if err := tx.Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Where(&model.RoleMenu{RoleID: roleId}).Delete(&model.RoleMenu{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if len(menuIds) > 0 {
		for _, mid := range menuIds {
			rm := new(model.RoleMenu)
			rm.RoleID = roleId
			rm.MenuID = mid
			if err := tx.Create(rm).Error; err != nil {
				tx.Rollback()
				return err
			}
		}
	}
	return tx.Commit().Error
}
