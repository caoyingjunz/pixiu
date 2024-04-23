package sqlite

import (
	"fmt"

	sqliteDriver "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db/dbconn"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

func NewDb(sqlConfig config.SqliteOptions, mode string, migrate bool) (*dbconn.DbConn, error) {

	opt := &gorm.Config{}
	if mode == mode {
		opt.Logger = logger.Default.LogMode(logger.Info)
	}

	dsn := fmt.Sprintf("%s?charset=utf8&parseTime=True&loc=Local", sqlConfig.Db)
	DB, err := gorm.Open(sqliteDriver.Open(dsn), opt)
	sqlDB, err := DB.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(types.MaxIdleConns)
	sqlDB.SetMaxOpenConns(types.MaxOpenConns)
	if migrate {
		if err := newMigrator(DB).AutoMigrate(); err != nil {
			return nil, err
		}
	}

	return &dbconn.DbConn{
		Conn: DB,
	}, nil
}
