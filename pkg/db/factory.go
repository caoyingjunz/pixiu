package db

import "gorm.io/gorm"

import (
	"errors"
)

var (
	ErrRecordNotUpdate = errors.New("record not updated")
)

func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

func IsNotUpdate(err error) bool {
	return errors.Is(err, ErrRecordNotUpdate)
}

type ShareDaoFactory interface {
}

type shareDaoFactory struct {
	db *gorm.DB
}

func NewDaoFactory(db *gorm.DB) ShareDaoFactory {
	return &shareDaoFactory{
		db: db,
	}
}
