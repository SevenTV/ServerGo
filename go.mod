module github.com/SevenTV/ServerGo

go 1.16

require (
	github.com/antonfisher/nested-logrus-formatter v1.3.1
	github.com/aws/aws-sdk-go v1.34.28
	github.com/bsm/redislock v0.7.0
	github.com/bwmarrin/discordgo v0.23.2
	github.com/davecgh/go-spew v1.1.1
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/fasthttp/websocket v1.4.3-beta.1 // indirect
	github.com/go-redis/redis/v8 v8.7.1
	github.com/gobuffalo/packr/v2 v2.8.1
	github.com/gofiber/fiber/v2 v2.7.1
	github.com/gofiber/websocket/v2 v2.0.3
	github.com/google/uuid v1.2.0
	github.com/graph-gophers/graphql-go v0.0.0-20210319060855-d2656e8bde15
	github.com/json-iterator/go v1.1.10
	github.com/karrick/godirwalk v1.16.1 // indirect
	github.com/kr/pretty v0.2.1
	github.com/pasztorpisti/qs v0.0.0-20171216220353-8d6c33ee906c
	github.com/rogpeppe/go-internal v1.8.0 // indirect
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.1
	go.mongodb.org/mongo-driver v1.5.0
	golang.org/x/crypto v0.0.0-20210513164829-c07d793c2f9a // indirect
	golang.org/x/image v0.0.0-20210504121937-7319ad40d33e
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c // indirect
	golang.org/x/sys v0.0.0-20210514084401-e8d321eab015 // indirect
	golang.org/x/term v0.0.0-20210317153231-de623e64d2a6 // indirect
	gopkg.in/gographics/imagick.v3 v3.4.0
	gopkg.in/yaml.v2 v2.4.0 // indirect
	honnef.co/go/tools v0.2.0 // indirect
)

replace github.com/graph-gophers/graphql-go => github.com/troydota/graphql-go v0.0.0-20210513105700-d1faf5042c4f

replace github.com/gofiber/fiber/v2 => github.com/SevenTV/fiber/v2 v2.6.1-0.20210513111059-44313cd6b916
