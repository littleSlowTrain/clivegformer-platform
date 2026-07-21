package main

import (
	"context"
	"log"
	"net"

	filev1 "github.com/clivegformer/platform/contracts/gen/file/v1"
	storagev1 "github.com/clivegformer/platform/contracts/gen/storage/v1"
	"github.com/clivegformer/platform/file_srv/config"
	"github.com/clivegformer/platform/file_srv/handler"
	"github.com/clivegformer/platform/file_srv/initialize"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	cfg := config.Load()
	db, err := initialize.Database(cfg.DSN)
	if err != nil {
		log.Fatalf("connect mysql: %v", err)
	}
	cache := initialize.Redis(cfg.RedisAddr, cfg.RedisPassword)
	if err := cache.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("connect redis: %v", err)
	}
	storageConn, err := grpc.NewClient(cfg.StorageAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	serverImpl := handler.New(db, cache, storagev1.NewStorageServiceClient(storageConn), cfg.Bucket)
	consumerClient, err := initialize.StartUploadConsumer(cfg.RocketMQ, serverImpl)
	if err != nil {
		log.Fatalf("start rocketmq consumer: %v", err)
	}
	defer consumerClient.Shutdown()
	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		log.Fatal(err)
	}
	grpcServer := grpc.NewServer()
	filev1.RegisterFileServiceServer(grpcServer, serverImpl)
	log.Printf("file service listening on %s", cfg.GRPCAddr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal(err)
	}
}
