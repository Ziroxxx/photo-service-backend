package app

import (
	"photo-service-back/domain/user"
	"photo-service-back/transport/http/handlers"
	"photo-service-back/transport/http/middleware"

	"github.com/gin-gonic/gin"
)

type HTTPDeps struct {
	AuthHandler        *handlers.AuthHandler
	UserHandler        *handlers.UserHandler
	CompetitionHandler *handlers.CompetitionHandler
	AccessHandler      *handlers.AccessHandler
	PhotoHandler       *handlers.PhotoHandler
	AuthMiddleware     *middleware.AuthMiddleware
	MaintenanceHandler *handlers.MaintenanceHandler
}

func RegisterRoutes(r *gin.Engine, d HTTPDeps) {
	auth := r.Group("/auth")
	{
		auth.POST("/register", d.AuthHandler.Register)
		auth.POST("/login", d.AuthHandler.Login)
		auth.POST("/refresh", d.AuthHandler.Refresh)
		auth.POST("/logout", d.AuthHandler.Logout)
	}

	api := r.Group("/")
	api.Use(d.AuthMiddleware.RequireAuth())
	{
		api.GET("/me", d.AuthHandler.Me)
		userRead := api.Group("/users")
		userRead.Use(middleware.RequireRoles(user.RoleAdmin, user.RoleOrganizer))
		{
			userRead.GET("", d.UserHandler.List)
		}

		admin := api.Group("/users")
		admin.Use(middleware.RequireRoles(user.RoleAdmin))
		{
			//admin.GET("", d.UserHandler.List)
			admin.PATCH("/:id/role", d.UserHandler.PatchRole)
			admin.POST("/:id/role", d.UserHandler.PatchRole)
			admin.PATCH("/:id/status", d.UserHandler.PatchStatus)
			admin.POST("/:id/status", d.UserHandler.PatchStatus)
		}

		competitions := api.Group("/competitions")
		{
			competitions.GET("", d.CompetitionHandler.List)
			competitions.GET("/:id", d.CompetitionHandler.GetByID)
			competitions.GET("/:id/stages", d.CompetitionHandler.ListStages)
			competitions.GET("/:id/access", d.AccessHandler.GetCompetitionAccess)
			competitions.GET("/:id/photos", d.PhotoHandler.ListCompetitionPhotos)

			editor := competitions.Group("")
			editor.Use(middleware.RequireRoles(user.RoleAdmin, user.RoleOrganizer))
			{
				editor.POST("", d.CompetitionHandler.Create)
				editor.PATCH("/:id", d.CompetitionHandler.Update)
				editor.DELETE("/:id", d.CompetitionHandler.Delete)

				editor.POST("/:id/stages", d.CompetitionHandler.CreateStage)
				editor.PATCH("/:id/stages/:stageId", d.CompetitionHandler.UpdateStage)
				editor.DELETE("/:id/stages/:stageId", d.CompetitionHandler.DeleteStage)

				editor.POST("/:id/access", d.AccessHandler.CreateGrant)
				editor.PATCH("/:id/access/:grantId", d.AccessHandler.UpdateGrant)
				editor.DELETE("/:id/access/:grantId", d.AccessHandler.DeleteGrant)
			}

			uploader := competitions.Group("")
			uploader.Use(middleware.RequireRoles(user.RoleAdmin, user.RolePhotographer, user.RoleOrganizer))
			{
				uploader.POST("/:id/photos/upload", d.PhotoHandler.UploadPhotos)
			}
		}

		photos := api.Group("/photos")
		{
			photos.GET("/:id", d.PhotoHandler.GetPhotoByID)
			photos.PATCH("/:id", d.PhotoHandler.UpdatePhoto)
			photos.DELETE("/:id", d.PhotoHandler.DeletePhoto)

			photos.POST("/:id/bibs", d.PhotoHandler.AddBib)
			photos.DELETE("/:id/bibs/:bibId", d.PhotoHandler.DeleteBib)

			photos.GET("/:id/download", d.PhotoHandler.DownloadPhoto)
			photos.POST("/download", d.PhotoHandler.DownloadPhotos)
		}

		tech := api.Group("/tech")
		tech.Use(middleware.RequireRoles(user.RoleAdmin))
		{
			tech.POST("/reset-db", d.MaintenanceHandler.ResetDB)
			tech.POST("/reset-minio", d.MaintenanceHandler.ResetMinIO)
		}
	}
}
