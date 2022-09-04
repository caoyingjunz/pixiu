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
	"github.com/caoyingjunz/gopixiu/pkg/db/dbcommon"
	"github.com/caoyingjunz/gopixiu/pkg/log"
	"time"

	"gorm.io/gorm"

	dberrors "github.com/caoyingjunz/gopixiu/pkg/db/errors"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
)

type UserInterface interface {
	Create(ctx context.Context, obj *model.User) (*model.User, error)
	Update(ctx context.Context, uid int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, uid int64) error
	Get(ctx context.Context, uid int64) (*model.User, error)
	List(ctx context.Context) ([]model.User, error)

	GetByName(ctx context.Context, name string) (*model.User, error)
	GetRoleIDByUser(ctx context.Context, uid int64) (map[string][]int64, error)
	SetUserRoles(ctx context.Context, uid int64, rid []int64) error
	GetMenus(ctx context.Context, uid int64) (*[]model.Menu, error)
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

func (u *user) List(ctx context.Context) ([]model.User, error) {
	var us []model.User
	if err := u.db.Find(&us).Error; err != nil {
		return nil, err
	}

	return us, nil
}

func (u *user) GetByName(ctx context.Context, name string) (*model.User, error) {
	var obj model.User
	if err := u.db.Where("name = ?", name).First(&obj).Error; err != nil {
		return nil, err
	}
	return &obj, nil
}

func (u *user) SetUserRoles(ctx context.Context, uid int64, rids []int64) (err error) {
	tx := u.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	if err := tx.Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Where(&model.UserRole{UserID: uid}).Delete(&model.UserRole{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if len(rids) > 0 {
		for _, rid := range rids {
			rm := new(model.UserRole)
			rm.RoleID = rid
			rm.UserID = uid
			if err := tx.Create(rm).Error; err != nil {
				tx.Rollback()
				return err
			}
		}
	}

	return tx.Commit().Error
}

func (u *user) GetRoleIDByUser(ctx context.Context, uid int64) (map[string][]int64, error) {
	comm := dbcommon.NewCommon(u.db)
	where := model.UserRole{UserID: uid}
	roleList := []int64{}
	err := comm.PluckList(&model.UserRole{}, &where, &roleList, "role_id")
	if err != nil {

		return nil, err
	}
	roleInfo := map[string][]int64{
		"role_ids": roleList,
	}
	return roleInfo, nil
}

func (u *user) GetMenus(ctx context.Context, uid int64) (*[]model.Menu, error) {
	var menus []model.Menu
	err := u.db.Table("menu").Select(" menu.id, menu.parent_id,menu.name, menu.url, menu.icon,menu.sequence,menu.code,menu.method, menu.menu_type").
		Joins("left join role_menu on menu.id = role_menu.menu_id ").
		Joins("left join user_role on user_role.user_id = role_menu.role_id  and user_role.user_id = ?", uid).
		Where("menu.menu_type = 2").
		Order("parent_id ASC").
		Order("sequence ASC").
		Scan(&menus).Error
	if err != nil {
		log.Logger.Errorf(err.Error())
		return &menus, err
	}
	return &menus, nil
}
