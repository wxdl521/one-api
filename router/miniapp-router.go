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
			protected.GET("/me/overview", controller.MiniAppAccountOverview)
			protected.GET("/plans", controller.MiniAppListPlans)
			protected.GET("/products", controller.MiniAppListProducts)
			protected.GET("/orders", controller.MiniAppListOrders)
			protected.POST("/checkout", middleware.CriticalRateLimit(), controller.MiniAppStartCheckout)
			protected.GET("/tokens", controller.MiniAppListTokens)
			protected.POST("/tokens", middleware.MiniAppTokenRequestBodyLimit(), middleware.CriticalRateLimit(), middleware.DisableCache(), controller.MiniAppCreateToken)
			protected.PATCH("/tokens/:id/status", middleware.MiniAppTokenRequestBodyLimit(), middleware.CriticalRateLimit(), controller.MiniAppUpdateTokenStatus)
			protected.DELETE("/tokens/:id", middleware.CriticalRateLimit(), controller.MiniAppRevokeToken)
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

	browserCheckout := router.Group("/api/miniapp/checkout")
	browserCheckout.Use(middleware.RouteTag("api"))
	browserCheckout.Use(middleware.MiniAppFeatureGate())
	browserCheckout.Use(middleware.BodyStorageCleanup())
	browserCheckout.Use(middleware.MiniAppBindingRequestBodyLimit())
	browserCheckout.Use(middleware.UserAuth(), middleware.MiniAppAuthenticatedUserRateLimit(), middleware.CriticalRateLimit())
	{
		browserCheckout.POST("/confirm", controller.ConfirmMiniAppBrowserCheckout)
	}
}
