module github.com/clivegformer/platform/user_srv

go 1.22

require (
	github.com/clivegformer/platform/contracts v0.0.0
	github.com/golang-jwt/jwt/v5 v5.2.1
	golang.org/x/crypto v0.28.0
	google.golang.org/grpc v1.67.1
	gorm.io/driver/mysql v1.5.7
	gorm.io/gorm v1.25.12
)

require (
	github.com/go-sql-driver/mysql v1.7.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	golang.org/x/net v0.30.0 // indirect
	golang.org/x/sys v0.26.0 // indirect
	golang.org/x/text v0.19.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240814211410-ddb44dafa142 // indirect
	google.golang.org/protobuf v1.35.1 // indirect
)

replace github.com/clivegformer/platform/contracts => ../contracts
