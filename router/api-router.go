package router

import (
	"NewAPI-Gateway/controller"
	"NewAPI-Gateway/middleware"

	"github.com/gin-gonic/gin"
)

func SetApiRouter(router *gin.Engine) {
	apiRouter := router.Group("/api")
	apiRouter.Use(middleware.GlobalAPIRateLimit())
	{
		apiRouter.GET("/status", controller.GetStatus)
		apiRouter.GET("/notice", controller.GetNotice)
		apiRouter.GET("/about", controller.GetAbout)
		apiRouter.GET("/verification", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.SendEmailVerification)
		apiRouter.GET("/reset_password", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.SendPasswordResetEmail)
		apiRouter.POST("/user/reset", middleware.CriticalRateLimit(), controller.ResetPassword)
		apiRouter.GET("/oauth/github", middleware.CriticalRateLimit(), controller.GitHubOAuth)
		apiRouter.GET("/oauth/wechat", middleware.CriticalRateLimit(), controller.WeChatAuth)
		apiRouter.GET("/oauth/wechat/bind", middleware.CriticalRateLimit(), middleware.UserAuth(), controller.WeChatBind)
		apiRouter.GET("/oauth/email/bind", middleware.CriticalRateLimit(), middleware.UserAuth(), controller.EmailBind)

		userRoute := apiRouter.Group("/user")
		{
			userRoute.POST("/register", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.Register)
			userRoute.POST("/login", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.Login)
			userRoute.GET("/logout", controller.Logout)

			selfRoute := userRoute.Group("/")
			selfRoute.Use(middleware.UserAuth(), middleware.NoTokenAuth())
			{
				selfRoute.GET("/self", controller.GetSelf)
				selfRoute.PUT("/self", controller.UpdateSelf)
				selfRoute.DELETE("/self", controller.DeleteSelf)
				selfRoute.GET("/token", controller.GenerateToken)
			}

			adminRoute := userRoute.Group("/")
			adminRoute.Use(middleware.AdminAuth(), middleware.NoTokenAuth())
			{
				adminRoute.GET("/", controller.GetAllUsers)
				adminRoute.GET("/search", controller.SearchUsers)
				adminRoute.GET("/:id", controller.GetUser)
				adminRoute.POST("/", controller.CreateUser)
				adminRoute.POST("/manage", controller.ManageUser)
				adminRoute.PUT("/", controller.UpdateUser)
				adminRoute.DELETE("/:id", controller.DeleteUser)
			}
		}

		optionRoute := apiRouter.Group("/option")
		optionRoute.Use(middleware.RootAuth(), middleware.NoTokenAuth())
		{
			optionRoute.GET("/", controller.GetOptions)
			optionRoute.PUT("/", controller.UpdateOption)
		}

		// === Provider Management (Admin) ===
		providerRoute := apiRouter.Group("/provider")
		providerRoute.Use(middleware.AdminAuth(), middleware.NoTokenAuth())
		{
			providerRoute.GET("/", controller.GetProviders)
			providerRoute.GET("/export", controller.ExportProviders)
			providerRoute.POST("/import", controller.ImportProviders)
			providerRoute.GET("/checkin/summary", controller.GetCheckinRunSummaries)
			providerRoute.GET("/checkin/messages", controller.GetCheckinRunMessages)
			providerRoute.GET("/checkin/uncheckin", controller.GetUncheckinProviders)
			providerRoute.POST("/checkin/run", controller.TriggerFullCheckinRunHandler)
			providerRoute.GET("/:id", controller.GetProviderDetail)
			providerRoute.POST("/", controller.CreateProvider)
			providerRoute.PUT("/", controller.UpdateProvider)
			providerRoute.DELETE("/:id", controller.DeleteProvider)
			providerRoute.POST("/:id/sync", controller.SyncProviderHandler)
			providerRoute.POST("/:id/checkin", controller.CheckinProviderHandler)
			providerRoute.GET("/:id/tokens", controller.GetProviderTokens)
			providerRoute.GET("/:id/pricing", controller.GetProviderPricing)
			providerRoute.GET("/:id/model-alias-mapping", controller.GetProviderModelAliasMapping)
			providerRoute.PUT("/:id/model-alias-mapping", controller.UpdateProviderModelAliasMapping)
			providerRoute.POST("/:id/tokens", controller.CreateProviderToken)
			providerRoute.PUT("/token/:token_id", controller.UpdateProviderToken)
			providerRoute.DELETE("/token/:token_id", controller.DeleteProviderToken)
		}

		// === Aggregated Token (User) ===
		aggTokenRoute := apiRouter.Group("/agg-token")
		aggTokenRoute.Use(middleware.UserAuth(), middleware.NoTokenAuth())
		{
			aggTokenRoute.GET("/", controller.GetAggTokens)
			aggTokenRoute.POST("/", controller.CreateAggToken)
			aggTokenRoute.PUT("/", controller.UpdateAggToken)
			aggTokenRoute.DELETE("/:id", controller.DeleteAggToken)
		}

		// === Model Routes (Admin) ===
		routeGroup := apiRouter.Group("/route")
		routeGroup.Use(middleware.AdminAuth(), middleware.NoTokenAuth())
		{
			routeGroup.GET("/", controller.GetModelRoutes)
			routeGroup.GET("/overview", controller.GetModelRouteOverview)
			routeGroup.GET("/models", controller.GetAllModels)
			routeGroup.PUT("/:id", controller.UpdateRoute)
			routeGroup.POST("/batch-update", controller.BatchUpdateRoutes)
			routeGroup.POST("/rebuild", controller.RebuildRoutes)
		}

		// === Logs (User sees own, Admin sees all) ===
		logRoute := apiRouter.Group("/log")
		logRoute.Use(middleware.UserAuth(), middleware.NoTokenAuth())
		{
			logRoute.GET("/self", controller.GetSelfLogs)
		}
		adminLogRoute := apiRouter.Group("/log")
		adminLogRoute.Use(middleware.AdminAuth(), middleware.NoTokenAuth())
		{
			adminLogRoute.GET("/", controller.GetAllLogs)
		}

		// === Dashboard (Admin) ===
		apiRouter.GET("/dashboard", middleware.AdminAuth(), controller.GetDashboard)
	}
}
