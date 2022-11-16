/*
Copyright 2021 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package user

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	dberrors "github.com/caoyingjunz/gopixiu/pkg/errors"
	"github.com/caoyingjunz/gopixiu/pkg/log"
)

type UserInterface interface {
	Create(ctx context.Context, obj *model.User) (*model.User, error)
	Update(ctx context.Context, uid int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, uid int64) error
	Get(ctx context.Context, uid int64) (*model.User, error)
	List(ctx context.Context, page int, pageSize int) ([]model.User, int64, error)

	// UpdateInternal 忽略 resourceVersion 直接更新，用于内部更新
	UpdateInternal(ctx context.Context, uid int64, updates map[string]interface{}) error
	UpdateStatus(c context.Context, userId, status int64) error

	GetByName(ctx context.Context, name string) (*model.User, error)
	GetRoleIDByUser(ctx context.Context, uid int64) (*[]model.Role, error)
	SetUserRoles(ctx context.Context, uid int64, rid []int64) error
	GetButtonsByUserID(ctx context.Context, uid int64) (*[]model.Menu, error)
	GetLeftMenusByUserID(ctx context.Context, uid int64) (*[]model.Menu, error)
}

type user struct {
	db *gorm.DB
}

func NewUser(db *gorm.DB) UserInterface {
	return &user{db}
}

func (u *user) Create(ctx context.Context, obj *model.User) (*model.User, error) {
	// 系统维护字段
	now := time.Now()
	obj.GmtCreate = now
	obj.GmtModified = now

	if err := u.db.Create(obj).Error; err != nil {
		return nil, err
	}
	return obj, nil
}

func (u *user) Update(ctx context.Context, uid int64, resourceVersion int64, updates map[string]interface{}) error {
	// 系统维护字段
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1

	f := u.db.Model(&model.User{}).
		Where("id = ? and resource_version = ?", uid, resourceVersion).
		Updates(updates)
	if f.Error != nil {
		return f.Error
	}

	if f.RowsAffected == 0 {
		return dberrors.ErrRecordNotUpdate
	}

	return nil
}

func (u *user) Delete(ctx context.Context, uid int64) error {
	return u.db.
		Where("id = ?", uid).
		Delete(&model.User{}).
		Error
}

func (u *user) Get(ctx context.Context, uid int64) (*model.User, error) {
	var obj model.User
	if err := u.db.Where("id = ?", uid).First(&obj).Error; err != nil {
		return nil, err
	}

	return &obj, nil
}

// List 分页查询
func (u *user) List(ctx context.Context, page, pageSize int) ([]model.User, int64, error) {
	var (
		us    []model.User
		total int64
	)
	if err := u.db.Limit(pageSize).Offset((page - 1) * pageSize).Find(&us).Error; err != nil {
		return nil, 0, err
	}
	if err := u.db.Model(&model.User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	return us, total, nil
}

func (u *user) UpdateStatus(c context.Context, userId, status int64) error {
	return u.db.Model(&model.User{}).Where("id = ?", userId).Update("status", status).Error
}

func (u *user) UpdateInternal(ctx context.Context, uid int64, updates map[string]interface{}) error {
	// 系统维护字段
	updates["gmt_modified"] = time.Now()

	f := u.db.Model(&model.User{}).Where("id = ?", uid).Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return dberrors.ErrRecordNotUpdate
	}

	return nil
}

func (u *user) GetByName(ctx context.Context, name string) (*model.User, error) {
	var obj model.User
	if err := u.db.Where("name = ?", name).First(&obj).Error; err != nil {
		return nil, err
	}

	return &obj, nil
}

// SetUserRoles 分配用户角色
func (u *user) SetUserRoles(ctx context.Context, uid int64, rids []int64) (err error) {
	tx := u.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			log.Logger.Errorf(err.(error).Error())
			tx.Rollback()
		}
	}()

	if err := tx.Error; err != nil {
		log.Logger.Errorf(err.Error())
		tx.Rollback()
		return err
	}

	if err := tx.Where(&model.UserRole{UserID: uid}).Delete(&model.UserRole{}).Error; err != nil {
		log.Logger.Errorf(err.Error())
		tx.Rollback()
		return err
	}
	if len(rids) > 0 {
		for _, rid := range rids {
			rm := new(model.UserRole)
			rm.RoleID = rid
			rm.UserID = uid
			if err := tx.Create(rm).Error; err != nil {
				log.Logger.Errorf(err.Error())
				tx.Rollback()
				return err
			}
		}
	}

	return tx.Commit().Error
}

// GetRoleIDByUser 查询用户角色
func (u *user) GetRoleIDByUser(ctx context.Context, uid int64) (roles *[]model.Role, err error) {
	subRoleIdSql := u.db.Select("role_id").Where("user_id = ?", uid).Table("user_roles")
	err = u.db.Table("roles").
		Select("roles.*").
		Joins("left join user_roles on roles.id = user_roles.role_id").
		Where("roles.id in (?)", subRoleIdSql).
		Or("roles.parent_id in (?)", subRoleIdSql).
		Group("id").
		Order("id asc").
		Order("sequence desc").
		Scan(&roles).Error
	if err != nil {
		log.Logger.Errorf(err.Error())
		return nil, err
	}

	if roles != nil {
		res := getTreeRoles(*roles, 0)
		return &res, err
	}

	return nil, err
}

// GetButtonsByUserID 获取菜单按钮
func (u *user) GetButtonsByUserID(ctx context.Context, uid int64) (*[]model.Menu, error) {
	var permissions []model.Menu

	err := u.db.Debug().Table("menus").Select(" menus.id, menus.code,menus.menu_type,menus.status").
		Joins("left join role_menus on menus.id = role_menus.menu_id ").
		Joins("left join user_roles on user_roles.role_id = role_menus.role_id where role_menus.role_id in (?) and menus.menu_type in (2,3) and menus.status = 1",
			u.db.Table("roles").Select("roles.id").
				Joins("left join user_roles on user_roles.role_id = roles.id where  user_roles.user_id = ? and roles.status = 1", uid)).
		Group("id").
		Scan(&permissions).Error
	if err != nil {
		return nil, err
	}
	return &permissions, nil
}

// GetLeftMenusByUserID 根据用户ID获取左侧菜单
func (u *user) GetLeftMenusByUserID(ctx context.Context, uid int64) (*[]model.Menu, error) {
	var menus []model.Menu
	err := u.db.Debug().Table("menus").Select(" menus.id, menus.parent_id,menus.name,menus.memo, menus.url, menus.icon,menus.sequence,"+
		"menus.method, menus.menu_type, menus.status").
		Joins("left join role_menus on menus.id = role_menus.menu_id where role_menus.role_id in (?) and menus.menu_type = 1 and menus.status = 1",
			u.db.Table("roles").Select("roles.id").
				Joins("left join user_roles on user_roles.role_id = roles.id where  user_roles.user_id = ? and roles.status = 1", uid)).
		Group("id").
		Order("parent_id ASC").
		Order("sequence DESC").
		Scan(&menus).Error

	if err != nil {
		log.Logger.Error(err)
		return nil, err
	}
	if len(menus) == 0 {
		return &menus, nil
	}

	treeMenusList := getTreeMenus(menus, 0)
	return &treeMenusList, nil
}
