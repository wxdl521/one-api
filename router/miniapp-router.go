package router

import (
	"github.com/QuantumNous/the-one/controller"
	"github.com/QuantumNous/the-one/middleware"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

// SetMiniAppRouter mounts the Mini Program BFF separately from both the
// dashboard API and relay APIs. Browser binding confirmation remains in the
// protected /api family and uses normal dashboard authentication.
func SetMiniAppRouter(router *gin.Engine) {
	miniAppRouter := router.Group("/api/miniapp/v1")
	miniAppRouter.Use(middleware.RouteTag("miniapp"))
	miniAppRouter.Use(middleware.MiniAppFeatureGate())
	miniAppRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	miniAppRouter.Use(middleware.BodyStorageCleanup())
	miniAppRouter.Use(middleware.GlobalAPIRateLimit())
	anonymousRequestBodyLimit := middleware.AnonymousRequestBodyLimit()
	{
		miniAppRouter.POST("/auth/wechat", middleware.MiniAppAnonymousIPRateLimit(), anonymousRequestBodyLimit, middleware.TurnstileCheck(), controller.MiniAppWechatLogin)
		miniAppRouter.POST("/auth/register", middleware.MiniAppAnonymousIPRateLimit(), anonymousRequestBodyLimit, middleware.TurnstileCheck(), controller.MiniAppRegister)
		miniAppRouter.POST("/bindings", middleware.MiniAppAnonymousIPRateLimit(), anonymousRequestBodyLimit, controller.MiniAppStartBinding)
		miniAppRouter.GET("/bindings/:id", middleware.MiniAppAnonymousIPRateLimit(), controller.MiniAppBindingStatus)
		miniAppRouter.POST("/auth/renew", middleware.MiniAppAnonymousIPRateLimit(), anonymousRequestBodyLimit, middleware.TurnstileCheck(), controller.MiniAppRenewLogin)

		protected := miniAppRouter.Group("/")
		protected.Use(middleware.MiniAppAuth(), middleware.MiniAppAuthenticatedUserRateLimit())
		{
			protected.POST("/auth/logout", controller.MiniAppLogout)
		}
	}

	browserBinding := router.Group("/api/miniapp/bindings")
	browserBinding.Use(middleware.RouteTag("api"))
	browserBinding.Use(middleware.MiniAppFeatureGate())
	browserBinding.Use(middleware.BodyStorageCleanup())
	browserBinding.Use(middleware.MiniAppBindingRequestBodyLimit())
	browserBinding.Use(middleware.UserAuth(), middleware.MiniAppAuthenticatedUserRateLimit())
	{
		browserBinding.POST("/confirm", controller.ConfirmMiniAppBrowserBinding)
	}
}
