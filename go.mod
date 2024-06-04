module github.com/caoyingjunz/pixiu

go 1.16

require (
	github.com/BurntSushi/toml v1.2.0 // indirect
	github.com/bmatcuk/doublestar/v4 v4.6.1
	github.com/caoyingjunz/pixiulib v0.0.0-20220819163605-c3c10ec3ed3c
	github.com/docker/docker v17.12.0-ce-rc1.0.20200618181300-9dc6525e6118+incompatible
	github.com/gin-contrib/cors v1.4.0
	github.com/gin-contrib/requestid v0.0.6
	github.com/gin-gonic/gin v1.8.1
	github.com/go-openapi/spec v0.20.7 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/go-playground/locales v0.14.1
	github.com/go-playground/universal-translator v0.18.1
	github.com/go-playground/validator/v10 v10.19.0
	github.com/go-sql-driver/mysql v1.6.0
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.4.2
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.3.0
	github.com/gorilla/websocket v1.4.2
	github.com/imdario/mergo v0.3.13 // indirect
	github.com/juju/ratelimit v1.0.2
	github.com/lib/pq v1.10.2 // indirect
	github.com/mattn/go-colorable v0.1.6 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-sqlite3 v1.14.12 // indirect
	github.com/mittwald/go-helm-client v0.8.1
	github.com/pelletier/go-toml/v2 v2.2.0 // indirect
	github.com/rogpeppe/go-internal v1.8.1 // indirect
	github.com/sirupsen/logrus v1.9.3
	github.com/spf13/cobra v1.5.0
	github.com/stretchr/testify v1.9.0 // indirect
	github.com/swaggo/files v0.0.0-20220728132757-551d4a08d97a
	github.com/swaggo/gin-swagger v1.5.3
	github.com/swaggo/swag v1.8.6
	github.com/ugorji/go/codec v1.2.12 // indirect
	golang.org/x/crypto v0.21.0
	golang.org/x/net v0.22.0 // indirect
	golang.org/x/oauth2 v0.1.0 // indirect
	golang.org/x/sync v0.1.0
	golang.org/x/sys v0.19.0 // indirect
	golang.org/x/time v0.1.0
	google.golang.org/protobuf v1.33.0 // indirect
	gorm.io/driver/mysql v1.3.6
	gorm.io/gorm v1.23.8
	helm.sh/helm/v3 v3.6.3
	k8s.io/api v0.22.1
	k8s.io/apimachinery v0.22.1
	k8s.io/client-go v0.22.1
	k8s.io/klog/v2 v2.80.1
	k8s.io/metrics v0.21.0
	k8s.io/utils v0.0.0-20221012122500-cfd413dd9e85 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

replace (
	github.com/pelletier/go-toml/v2 => github.com/pelletier/go-toml/v2 v2.1.1
	golang.org/x/net => golang.org/x/net v0.17.0
)
