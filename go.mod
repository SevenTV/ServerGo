module github.com/SevenTV/ServerGo

go 1.16

require (
	github.com/andybalholm/brotli v1.0.3 // indirect
	github.com/aws/aws-sdk-go v1.40.22
	github.com/bsm/redislock v0.7.1
	github.com/bwmarrin/discordgo v0.23.2
	github.com/davecgh/go-spew v1.1.1
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/fsnotify/fsnotify v1.4.9
	github.com/go-redis/redis/v8 v8.11.3
	github.com/gobuffalo/packr/v2 v2.8.1
	github.com/gofiber/fiber/v2 v2.17.0
	github.com/google/uuid v1.3.0
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/graph-gophers/graphql-go v0.0.0-20210319060855-d2656e8bde15
	github.com/hashicorp/go-multierror v1.1.1
	github.com/json-iterator/go v1.1.11
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/karrick/godirwalk v1.16.1 // indirect
	github.com/klauspost/compress v1.13.0 // indirect
	github.com/kr/pretty v0.3.0
	github.com/mitchellh/panicwrap v1.0.0
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/pasztorpisti/qs v0.0.0-20171216220353-8d6c33ee906c
	github.com/rogpeppe/go-internal v1.8.0 // indirect
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.8.1
	go.mongodb.org/mongo-driver v1.7.1
	golang.org/x/sys v0.0.0-20210603125802-9665404d3644 // indirect
	golang.org/x/term v0.0.0-20210317153231-de623e64d2a6 // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/gographics/imagick.v3 v3.4.0
)

replace github.com/graph-gophers/graphql-go => github.com/troydota/graphql-go v0.0.0-20210702180404-92fc941a47cf

replace github.com/mitchellh/panicwrap => github.com/bugsnag/panicwrap v1.3.3
