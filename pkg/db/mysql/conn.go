package mysql

import (
	"fmt"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db/dbconn"
	"github.com/caoyingjunz/pixiu/pkg/types"
	mysqlDriver "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewDb(sqlConfig config.MysqlOptions, mode string, migrate bool) (*dbconn.DbConn, error) {

	opt := &gorm.Config{}
	if mode == mode {
		opt.Logger = logger.Default.LogMode(logger.Info)
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8&parseTime=True&loc=Local",
		sqlConfig.User,
		sqlConfig.Password,
		sqlConfig.Host,
		sqlConfig.Port,
		sqlConfig.Name)
	DB, err := gorm.Open(mysqlDriver.Open(dsn), opt)
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
