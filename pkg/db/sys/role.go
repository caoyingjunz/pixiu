package sys

import (
	"context"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	"github.com/caoyingjunz/gopixiu/pkg/log"
	"gorm.io/gorm"
)

// RoleInterface 角色操作接口
type RoleInterface interface {
	Create(context.Context, *model.Role) (*model.Role, error)
	Update(context.Context, *model.Role) error
	Delete(context.Context, int64) error
	Get(context.Context, int64) (*model.Role, error)
	List(context.Context) ([]model.Role, error)

	GetByName(context.Context, string) (*model.Role, error)
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

func (r *role) Update(c context.Context, role *model.Role) error {
	return r.db.Updates(*role).Error
}

func (r *role) Delete(c context.Context, rid int64) error {
	tx := r.db.Begin()
	defer func() {
		if err := recover(); err != nil {
			tx.Rollback()
		}
	}()
	if err := tx.Error; err != nil {
		tx.Rollback()
		return err
	}
	// 删除角色
	if err := tx.Where("id = ?", rid).Delete(&model.Role{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	//删除角色相关的菜单
	if err := tx.Where("role_id = ?", rid).Delete(&model.RoleMenu{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (r *role) Get(c context.Context, rid int64) (role *model.Role, err error) {
	if err := r.db.Where("id = ?", rid).First(&role).Error; err != nil {
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

func (r *role) GetByName(c context.Context, name string) (role *model.Role, err error) {
	if err := r.db.Where("name = ?", name).First(&role).Error; err != nil {
		return nil, err
	}
	return
}
