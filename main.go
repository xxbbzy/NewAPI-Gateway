package main

import (
	"NewAPI-Gateway/common"
	"NewAPI-Gateway/middleware"
	"NewAPI-Gateway/model"
	"NewAPI-Gateway/router"
	"NewAPI-Gateway/service"
	"embed"
	"log"
	"os"
	"strconv"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-contrib/sessions/redis"
	"github.com/gin-gonic/gin"
)

//go:embed web/build
var buildFS embed.FS

//go:embed web/build/index.html
var indexPage []byte

func main() {
	common.SetupGinLog()
	common.SysLog("NewAPI Gateway " + common.Version + " started")
	if os.Getenv("GIN_MODE") != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	// Initialize SQL Database
	err := model.InitDB()
	if err != nil {
		common.FatalLog(err)
	}
	defer func() {
		err := model.CloseDB()
		if err != nil {
			common.FatalLog(err)
		}
	}()

	// Initialize Redis
	err = common.InitRedisClient()
	if err != nil {
		common.FatalLog(err)
	}

	// Initialize options
	model.InitOptionMap()
	if err = model.RunRelayReliabilityStartupPreflight(); err != nil {
		common.FatalLog(err)
	}

	// Start cron jobs (sync & checkin)
	service.StartCronJobs()
	defer service.StopCronJobs()

	// Initialize HTTP server
	server := gin.Default()
	server.Use(middleware.CORS())

	// Initialize session store
	if common.RedisEnabled {
		opt := common.ParseRedisOption()
		store, _ := redis.NewStore(opt.MinIdleConns, opt.Network, opt.Addr, opt.Password, []byte(common.SessionSecret))
		server.Use(sessions.Sessions("session", store))
	} else {
		store := cookie.NewStore([]byte(common.SessionSecret))
		server.Use(sessions.Sessions("session", store))
	}

	router.SetRouter(server, buildFS, indexPage)
	var port = os.Getenv("PORT")
	if port == "" {
		port = strconv.Itoa(*common.Port)
	}
	err = server.Run(":" + port)
	if err != nil {
		log.Println(err)
	}
}
