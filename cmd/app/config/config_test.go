package config

import "testing"

func TestDatabaseDriver_DefaultToMySQL(t *testing.T) {
	c := &Config{}
	if got := c.DatabaseDriver(); got != DBDriverMySQL {
		t.Fatalf("expected default driver %q, got %q", DBDriverMySQL, got)
	}
}

func TestDatabaseOptions_ValidateSQLite(t *testing.T) {
	tests := []struct {
		name    string
		opts    DatabaseOptions
		wantErr bool
	}{
		{
			name: "sqlite with path",
			opts: DatabaseOptions{
				Driver: DBDriverSQLite,
				SQLite: SQLiteOptions{Path: "/tmp/pixiu.db"},
			},
			wantErr: false,
		},
		{
			name: "sqlite without path",
			opts: DatabaseOptions{
				Driver: DBDriverSQLite,
			},
			wantErr: true,
		},
		{
			name: "unsupported driver",
			opts: DatabaseOptions{
				Driver: "postgres",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Valid()
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestEffectiveMysql_UseNewConfigFirst(t *testing.T) {
	c := &Config{
		Mysql: MysqlOptions{
			Host: "legacy",
			User: "root",
		},
		Database: DatabaseOptions{
			Mysql: MysqlOptions{
				Host: "new-host",
				User: "new-user",
			},
		},
	}

	got := c.EffectiveMysql()
	if got.Host != "new-host" || got.User != "new-user" {
		t.Fatalf("expected mysql options from database.mysql, got %+v", got)
	}
}
