package options

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/caoyingjunz/pixiu/cmd/app/config"
)

func TestRegisterDatabase_SQLite(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "data", "pixiu.db")

	o := &Options{
		ComponentConfig: config.Config{
			Default: config.DefaultOptions{
				AutoMigrate: true,
			},
			Database: config.DatabaseOptions{
				Driver: config.DBDriverSQLite,
				SQLite: config.SQLiteOptions{
					Path: dbPath,
				},
			},
		},
	}

	if err := o.registerDatabase(); err != nil {
		t.Fatalf("register sqlite database failed: %v", err)
	}
	if o.db == nil {
		t.Fatalf("gorm db is nil after register")
	}
	if o.Factory == nil {
		t.Fatalf("dao factory is nil after register")
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("sqlite database file not created: %v", err)
	}
}
