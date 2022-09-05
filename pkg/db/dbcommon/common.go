package dbcommon

import (
	"errors"
	"gorm.io/gorm"
)

// CommonInterface 数据库通用接口
type CommonInterface interface {
	Create(value interface{}) error
	Save(value interface{}) error
	Updates(where interface{}, value interface{}) error
	DeleteByModel(model interface{}) (count int64, err error)
	DeleteByWhere(model, where interface{}) (count int64, err error)
	DeleteByID(model interface{}, id int64) (count int64, err error)
	DeleteByIDS(model interface{}, ids []int64) (count int64, err error)
	FirstByID(out interface{}, id int64) (notFound bool, err error)
	First(where interface{}, out interface{}) (notFound bool, err error)
	Find(where interface{}, out interface{}, orders ...string) error
	Scan(model, where interface{}, out interface{}) (notFound bool, err error)
	ScanList(model, where interface{}, out interface{}, orders ...string) error
	PluckList(model, where interface{}, out interface{}, fieldName string) error
}

type common struct {
	db *gorm.DB
}

func NewCommon(db *gorm.DB) *common {
	return &common{db}
}

// Create 通用查询接口
func (c *common) Create(value interface{}) error {
	return c.db.Create(value).Error
}

// Save 保存
func (c *common) Save(value interface{}) error {
	return c.db.Save(value).Error
}

// Updates 更新接口
func (c *common) Updates(where interface{}, value interface{}) error {
	return c.db.Model(where).Updates(value).Error
}

// DeleteByModel 删除接口
func (c *common) DeleteByModel(model interface{}) (count int64, err error) {
	db := c.db.Delete(model)
	err = db.Error
	if err != nil {
		return
	}
	count = db.RowsAffected
	return
}

// DeleteByWhere 条件删除接口
func (c *common) DeleteByWhere(model, where interface{}) (count int64, err error) {
	db := c.db.Where(where).Delete(model)
	err = db.Error
	if err != nil {
		return
	}
	count = db.RowsAffected
	return
}

// DeleteByID 根据ID删除
func (c *common) DeleteByID(model interface{}, id int64) (count int64, err error) {
	db := c.db.Where("id=?", id).Delete(model)
	err = db.Error
	if err != nil {
		return
	}
	count = db.RowsAffected
	return
}

// DeleteByIDS 批量删除
func (c *common) DeleteByIDS(model interface{}, ids []int64) (count int64, err error) {
	db := c.db.Where("id in (?)", ids).Delete(model)
	err = db.Error
	if err != nil {
		return
	}
	count = db.RowsAffected
	return
}

// FirstByID 查询接口
func (c *common) FirstByID(out interface{}, id int64) (notFound bool, err error) {
	err = c.db.First(out, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		notFound = true
	}
	return
}

// First 查询接口
func (c *common) First(where interface{}, out interface{}) (notFound bool, err error) {
	err = c.db.Where(where).First(out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		notFound = true
	}
	return
}

// Find  查询接口
func (c *common) Find(where interface{}, out interface{}, orders ...string) error {
	db := c.db.Where(where)
	if len(orders) > 0 {
		for _, order := range orders {
			db = db.Order(order)
		}
	}
	return db.Find(out).Error
}

// Scan 查询接口
func (c *common) Scan(model, where interface{}, out interface{}) (notFound bool, err error) {
	err = c.db.Model(model).Where(where).Scan(out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		notFound = true
	}

	return
}

// ScanList 查询接口
func (c *common) ScanList(model, where interface{}, out interface{}, orders ...string) error {
	db := c.db.Model(model).Where(where)
	if len(orders) > 0 {
		for _, order := range orders {
			db = db.Order(order)
		}
	}
	return db.Scan(out).Error
}

// PluckList 查询
func (c *common) PluckList(model, where interface{}, out interface{}, fieldName string) error {
	return c.db.Model(model).Where(where).Pluck(fieldName, out).Error
}
