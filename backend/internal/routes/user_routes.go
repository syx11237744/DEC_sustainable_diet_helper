// routes/user_routes.go
package routes

import (
    "gorm.io/gorm"
    "github.com/Alchuang22-dev/DEC_sustainable_diet_helper/internal/controllers"
    "github.com/Alchuang22-dev/DEC_sustainable_diet_helper/internal/middleware"
    "github.com/Alchuang22-dev/DEC_sustainable_diet_helper/internal/utils"

    "github.com/gin-gonic/gin"
)

func RegisterUserRoutes(router *gin.Engine, db *gorm.DB, utils utils.UtilsInterface) {
    userController := controllers.NewUserController(db, utils)

    userGroup := router.Group("/users")
    {
        // 公共路由
        userGroup.POST("/auth", userController.WeChatAuth) // 注册
        userGroup.POST("/refresh", userController.RefreshTokenHandler) // 刷新令牌
        userGroup.POST("/logout", userController.LogoutHandler) // 登出

        // 需要认证的路由
        authGroup := userGroup.Group("")
        authGroup.Use(middleware.AuthMiddleware())
        {
            authGroup.PUT("/set_nickname", userController.SetNickname) // 更新用户名
            authGroup.POST("/set_avatar", userController.SetAvatar) // 更新头像
            authGroup.GET("/basic_details", userController.UserBasicDetails) // 获取基本信息

            authGroup.GET("/liked", userController.GetMyLikedNews)
            authGroup.GET("/favorited", userController.GetMyFavoritedNews)
            authGroup.GET("/viewed", userController.GetMyViewedNews)

            authGroup.GET("/:id/profile", userController.GetUserProfile)
        }
    }
}