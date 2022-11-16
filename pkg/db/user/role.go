package user

import (
	"context"
	"errors"

	"github.com/fatih/structs"
	"gorm.io/gorm"

	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
)

// RoleInterface 角色操作接口
type RoleInterface interface {
	Create(context.Context, *model.Role) (*model.Role, error)
	Update(context.Context, *types.UpdateRoleReq, int64) error
	Delete(context.Context, int64) error
	Get(context.Context, int64) (*[]model.Role, error)
	List(c context.Context, page, limit int) (res *model.PageRole, err error)

	GetMenusByRoleID(c context.Context, rid int64) (*[]model.Menu, error)
	SetRole(ctx context.Context, roleId int64, menuIds []int64) error
	GetRolesByMenuID(ctx context.Context, menuId int64) (*[]int64, error)
	GetRoleByRoleName(ctx context.Context, roleName string) (*model.Role, error)
	UpdateStatus(c context.Context, roleId, status int64) error
}

type role struct {
	db *gorm.DB
}

func NewRole(db *gorm.DB) *role {
	return &role{db}
}

func (r *role) Create(c context.Context, obj *model.Role) (*model.Role, error) {
	if err := r.db.Create(obj).Error; err != nil {
		return nil, err
	}

	return obj, nil
}

func (r *role) Update(c context.Context, role *types.UpdateRoleReq, rid int64) error {
	resourceVersion := role.ResourceVersion
	role.ResourceVersion++
	roleMap := structs.Map(role)
	tx := r.db.Model(&model.Role{}).Where("id = ? and resource_version = ? ", rid, resourceVersion).Updates(roleMap)
	if tx.RowsAffected == 0 {
		return errors.New("update failed")
	}
	return tx.Error
}

func (r *role) Delete(c context.Context, rId int64) error {
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

	//删除角色相关的菜单
	if err := tx.Where("role_id = ?", rId).Delete(&model.RoleMenu{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 删除角色及其子角色
	if err := tx.Where("id  = ?", rId).
		Or("parent_id  = ?", rId).
		Delete(&model.Role{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 删除用户绑定的角色信息(用户需要重新绑定角色)
	if err := tx.Where("role_id = ?", rId).
		Or("parent_id = ?", rId).
		Delete(&model.UserRole{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

func (r *role) Get(c context.Context, rid int64) (roles *[]model.Role, err error) {
	err = r.db.Where("id = ?", rid).
		Or("parent_id = ?", rid).
		Order("sequence DESC").
		First(&roles).Error

	if err != nil {
		return nil, err
	}

	res := getTreeRoles(*roles, 0)

	return &res, err
}

func (r *role) List(c context.Context, page, limit int) (res *model.PageRole, err error) {
	var (
		roleList []model.Role
		total    int64
	)
	// 全量查询
	if page == 0 && limit == 0 {
		if tx := r.db.Order("sequence DESC").Find(&roleList); tx.Error != nil {
			return nil, tx.Error
		}
		treeRole := getTreeRoles(roleList, 0)

		if err := r.db.Model(&model.Role{}).Count(&total).Error; err != nil {
			return nil, err
		}

		res = &model.PageRole{
			Roles: treeRole,
			Total: total,
		}
		return res, err
	}

	//分页数据
	if err := r.db.Order("sequence DESC").Where("parent_id = 0").Limit(limit).Offset((page - 1) * limit).
		Find(&roleList).Error; err != nil {
		return nil, err
	}

	var roleIds []int64
	for _, role := range roleList {
		roleIds = append(roleIds, role.Id)
	}

	// 查询子角色
	if len(roleIds) != 0 {
		var roles []model.Role
		if err := r.db.Where("parent_id in ?", roleIds).Find(&roles).Error; err != nil {
			return nil, err
		}
		roleList = append(roleList, roles...)
	}

	if err := r.db.Model(&model.Role{}).Where("parent_id = 0").Count(&total).Error; err != nil {
		return nil, err
	}

	treeRoles := getTreeRoles(roleList, 0)
	res = &model.PageRole{
		Roles: treeRoles,
		Total: total,
	}

	return
}

func (r *role) GetMenusByRoleID(c context.Context, rid int64) (*[]model.Menu, error) {
	var menus []model.Menu
	err := r.db.Table("menus").Select(" menus.id, menus.parent_id,menus.name, menus.url, menus.icon,menus.sequence,menus.code,menus.method").
		Joins("left join role_menus on menus.id = role_menus.menu_id", rid).
		Where("role_menus.role_id = ?", rid).
		Order("parent_id ASC").
		Order("sequence ASC").
		Scan(&menus).Error

	if err != nil {
		return nil, err
	}

	//res := getTreeMenus(menus, 0)
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

func (r *role) GetRolesByMenuID(ctx context.Context, menuId int64) (roleIds *[]int64, err error) {
	err = r.db.Where("menu_id = ?", menuId).Table("role_menus").Pluck("role_id", &roleIds).Error
	if err != nil {
		return
	}
	return
}

func (r *role) GetRoleByRoleName(ctx context.Context, roleName string) (role *model.Role, err error) {
	err = r.db.Where("name = ?", roleName).First(&role).Error
	return
}

func getTreeRoles(rolesList []model.Role, pid int64) (treeRolesList []model.Role) {
	for _, node := range rolesList {
		if node.ParentID == pid {
			child := getTreeRoles(rolesList, node.Id)
			node.Children = child
			treeRolesList = append(treeRolesList, node)
		}
	}
	return treeRolesList
}

func (r *role) UpdateStatus(c context.Context, roleId, status int64) error {
	return r.db.Model(&model.Role{}).Where("id = ?", roleId).Update("status", status).Error
}
