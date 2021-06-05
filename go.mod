module github.com/SevenTV/ServerGo

go 1.16

require (
	github.com/andybalholm/brotli v1.0.3 // indirect
	github.com/antonfisher/nested-logrus-formatter v1.3.1
	github.com/aws/aws-sdk-go v1.38.55
	github.com/bsm/redislock v0.7.0
	github.com/bwmarrin/discordgo v0.23.2
	github.com/davecgh/go-spew v1.1.1
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/fasthttp/websocket v1.4.3 // indirect
	github.com/go-redis/redis/v8 v8.10.0
	github.com/gobuffalo/logger v1.0.4 // indirect
	github.com/gobuffalo/packr/v2 v2.8.1
	github.com/gofiber/fiber/v2 v2.11.0
	github.com/gofiber/websocket/v2 v2.0.4
	github.com/google/uuid v1.2.0
	github.com/graph-gophers/graphql-go v1.1.0
	github.com/json-iterator/go v1.1.11
	github.com/karrick/godirwalk v1.16.1 // indirect
	github.com/klauspost/compress v1.13.0 // indirect
	github.com/kr/pretty v0.2.1
	github.com/kr/text v0.2.0 // indirect
	github.com/magiconair/properties v1.8.5 // indirect
	github.com/mitchellh/mapstructure v1.4.1 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/pasztorpisti/qs v0.0.0-20171216220353-8d6c33ee906c
	github.com/pelletier/go-toml v1.9.1 // indirect
	github.com/savsgio/gotils v0.0.0-20210520110740-c57c45b83e0a // indirect
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/afero v1.6.0 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.1
	github.com/valyala/fasthttp v1.26.0 // indirect
	github.com/youmark/pkcs8 v0.0.0-20201027041543-1326539a0a0a // indirect
	go.mongodb.org/mongo-driver v1.5.3
	golang.org/x/sys v0.0.0-20210603125802-9665404d3644 // indirect
	golang.org/x/term v0.0.0-20210503060354-a79de5458b56 // indirect
	golang.org/x/tools v0.1.2 // indirect
	gopkg.in/gographics/imagick.v3 v3.4.0
	gopkg.in/ini.v1 v1.62.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	honnef.co/go/tools v0.2.0
)

replace github.com/graph-gophers/graphql-go => github.com/troydota/graphql-go v0.0.0-20210513105700-d1faf5042c4f

replace github.com/gofiber/fiber/v2 => github.com/SevenTV/fiber/v2 v2.6.1-0.20210513111059-44313cd6b916
