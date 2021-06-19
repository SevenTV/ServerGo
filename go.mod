module github.com/SevenTV/ServerGo

go 1.16

require (
	github.com/andybalholm/brotli v1.0.3 // indirect
	github.com/aws/aws-sdk-go v1.38.64
	github.com/bsm/redislock v0.7.0
	github.com/bwmarrin/discordgo v0.23.2
	github.com/davecgh/go-spew v1.1.1
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/fasthttp/websocket v1.4.3-beta.1 // indirect
	github.com/fsnotify/fsnotify v1.4.9
	github.com/go-redis/redis/v8 v8.7.1
	github.com/gobuffalo/packr/v2 v2.8.1
	github.com/gofiber/fiber/v2 v2.13.0
	github.com/gofiber/websocket/v2 v2.0.6
	github.com/google/uuid v1.2.0
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/graph-gophers/graphql-go v0.0.0-20210319060855-d2656e8bde15
	github.com/hashicorp/go-multierror v1.1.1
	github.com/json-iterator/go v1.1.11
	github.com/karrick/godirwalk v1.16.1 // indirect
	github.com/klauspost/compress v1.13.0 // indirect
	github.com/kr/pretty v0.2.1
	github.com/kr/text v0.2.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/pasztorpisti/qs v0.0.0-20171216220353-8d6c33ee906c
	github.com/rogpeppe/go-internal v1.8.0 // indirect
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.8.0
	go.mongodb.org/mongo-driver v1.5.0
	golang.org/x/net v0.0.0-20210525063256-abc453219eb5 // indirect
	golang.org/x/sys v0.0.0-20210603125802-9665404d3644 // indirect
	golang.org/x/term v0.0.0-20210317153231-de623e64d2a6 // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/gographics/imagick.v3 v3.4.0
	honnef.co/go/tools v0.0.1-2020.1.4
)

replace github.com/graph-gophers/graphql-go => github.com/troydota/graphql-go v0.0.0-20210513105700-d1faf5042c4f

replace github.com/gofiber/fiber/v2 => github.com/SevenTV/fiber/v2 v2.6.1-0.20210513111059-44313cd6b916
