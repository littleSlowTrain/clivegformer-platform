package main

import (
	"log"
	"net"

	userv1 "github.com/clivegformer/platform/contracts/gen/user/v1"
	"github.com/clivegformer/platform/user_srv/config"
	"github.com/clivegformer/platform/user_srv/handler"
	"github.com/clivegformer/platform/user_srv/initialize"
	"google.golang.org/grpc"
)

func main() {
	cfg := config.Load()
	db, err := initialize.Database(cfg.DSN)
	if err != nil {
		log.Fatalf("connect mysql: %v", err)
	}
	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	server := grpc.NewServer()
	userv1.RegisterUserServiceServer(server, handler.New(db, cfg.JWTSecret))
	log.Printf("user service listening on %s", cfg.GRPCAddr)
	if err := server.Serve(lis); err != nil {
		log.Fatal(err)
	}
}
