package util

import (
	"github.com/caoyingjunz/gopixiu/cmd/app/options"
	"github.com/google/uuid"
)

func GetUUID() string {
	cfg, _ := options.NewOptions()
	println("-----", cfg.ComponentConfig.Default.LogDir)
	return uuid.New().String()
}
