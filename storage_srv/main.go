package main

import (
	"log"
	"net"

	filev1 "github.com/clivegformer/platform/contracts/gen/file/v1"
	storagev1 "github.com/clivegformer/platform/contracts/gen/storage/v1"
	"github.com/clivegformer/platform/storage_srv/config"
	"github.com/clivegformer/platform/storage_srv/handler"
	"github.com/clivegformer/platform/storage_srv/initialize"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	cfg := config.Load()
	client, core, err := initialize.Ceph(cfg.Endpoint, cfg.AccessKey, cfg.SecretKey, cfg.Region, cfg.Secure)
	if err != nil {
		log.Fatal(err)
	}
	fileConn, err := grpc.NewClient(cfg.FileAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	events, err := initialize.NewEventProducer(cfg.RocketMQ)
	if err != nil {
		log.Fatalf("rocketmq producer: %v", err)
	}
	defer events.Shutdown()
	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		log.Fatal(err)
	}
	server := grpc.NewServer()
	storagev1.RegisterStorageServiceServer(server, handler.New(client, core, filev1.NewFileServiceClient(fileConn), events))
	log.Printf("storage service listening on %s, ceph=%s", cfg.GRPCAddr, cfg.Endpoint)
	if err := server.Serve(lis); err != nil {
		log.Fatal(err)
	}
}
