package main

import (
	"log"

	analysisv1 "github.com/clivegformer/platform/contracts/gen/analysis/v1"
	filev1 "github.com/clivegformer/platform/contracts/gen/file/v1"
	storagev1 "github.com/clivegformer/platform/contracts/gen/storage/v1"
	userv1 "github.com/clivegformer/platform/contracts/gen/user/v1"
	"github.com/clivegformer/platform/gin_web/api"
	"github.com/clivegformer/platform/gin_web/config"
	"github.com/clivegformer/platform/gin_web/router"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	cfg := config.Load()
	dial := func(address string) *grpc.ClientConn {
		conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatal(err)
		}
		return conn
	}
	userConn, fileConn, storageConn, analysisConn := dial(cfg.UserAddr), dial(cfg.FileAddr), dial(cfg.StorageAddr), dial(cfg.AnalysisAddr)
	handler := api.New(userv1.NewUserServiceClient(userConn), filev1.NewFileServiceClient(fileConn), storagev1.NewStorageServiceClient(storageConn), analysisv1.NewAnalysisServiceClient(analysisConn), cfg.JWTSecret)
	log.Printf("gin gateway listening on %s", cfg.HTTPAddr)
	if err := router.New(handler, cfg.JWTSecret, cfg.WebOrigin).Run(cfg.HTTPAddr); err != nil {
		log.Fatal(err)
	}
}
