package router

import (
	"net/http"
	"strings"

	"github.com/clivegformer/platform/gin_web/api"
	"github.com/clivegformer/platform/gin_web/middlewares"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func New(handler *api.Handler, jwtSecret, origin string) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), cors.New(cors.Config{AllowOrigins: splitOrigins(origin), AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}, AllowHeaders: []string{"Authorization", "Content-Type", "Range", "X-Chunk-SHA256"}, ExposeHeaders: []string{"Content-Range", "Content-Length", "Content-Disposition"}}))
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	r.GET("/readyz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ready"}) })
	v1 := r.Group("/api/v1")
	v1.POST("/auth/register", handler.Register)
	v1.POST("/auth/login", handler.Login)
	v1.GET("/download/:ticket", handler.Download)
	secured := v1.Group("")
	secured.Use(middlewares.Auth(jwtSecret))
	secured.POST("/uploads/initiate", handler.InitiateUpload)
	secured.GET("/uploads/:id", handler.GetUpload)
	secured.GET("/uploads/:id/parts", handler.GetUpload)
	secured.PUT("/uploads/:id/parts/:part", handler.UploadPart)
	secured.POST("/uploads/:id/complete", handler.CompleteUpload)
	secured.POST("/uploads/:id/pause", handler.PauseUpload)
	secured.POST("/uploads/:id/resume", handler.ResumeUpload)
	secured.POST("/uploads/:id/heartbeat", handler.HeartbeatUpload)
	secured.DELETE("/uploads/:id", handler.CancelUpload)
	secured.GET("/files", handler.ListFiles)
	secured.GET("/files/search", handler.ListFiles)
	secured.GET("/files/facets", handler.ListFileFacets)
	secured.GET("/files/:id", handler.GetFile)
	secured.POST("/files/:id/download-ticket", handler.DownloadTicket)
	secured.GET("/analysis/files/:id/variables", handler.Variables)
	secured.POST("/analysis/ndvi", handler.NDVI)
	secured.POST("/analysis/time-series", handler.TimeSeries)
	return r
}

func splitOrigins(value string) []string {
	result := make([]string, 0, 2)
	for _, item := range strings.Split(value, ",") {
		if origin := strings.TrimSpace(item); origin != "" {
			result = append(result, origin)
		}
	}
	return result
}
