package main

import (
	"context"
	"go-micro.dev/v6/service"
	pb "kerthus/gen/go/saas/v1"
	"kerthus/internal/platform/config"
	micro "kerthus/internal/platform/micro"
	"kerthus/internal/platform/rpcauth"
	"kerthus/internal/platform/storage"
	"kerthus/internal/saas/adapters/mysql"
	"kerthus/internal/saas/adapters/redis"
	rpc "kerthus/internal/saas/adapters/rpc"
	"kerthus/internal/saas/usecase/core"
	"log"
	"os/signal"
	"syscall"
)

func run() error {
	c, e := config.Load()
	if e != nil {
		return e
	}
	db, e := mysql.Open(c.DSN)
	if e != nil {
		return e
	}
	defer db.Close()
	sessions := redis.NewNamespaced(c.RedisAddress, c.RedisPassword, c.SessionNamespace)
	defer sessions.Close()
	objects, e := storage.New(c.StorageEndpoint, c.StorageAccessKey, c.StorageSecretKey, c.StorageBucket, c.StorageTLS)
	if e != nil {
		return e
	}
	platformService := core.New(db, sessions, objects)
	platformService.AttachmentMaxBytes = c.AttachmentMaxBytes
	handler, e := rpc.New(platformService, rpcauth.Config{GatewayKey: c.GatewayKey, AppKeys: c.AppKeys})
	if e != nil {
		return e
	}
	server, e := micro.Server(c)
	if e != nil {
		return e
	}
	if e = pb.RegisterPlatformHandler(server, handler); e != nil {
		return e
	}
	svc := service.New(service.Name(micro.ServiceName), service.Server(server), service.Registry(micro.Registry(c)))
	if e = svc.Start(); e != nil {
		return e
	}
	log.Printf("SaaS RPC listening on %s", c.RPCAddress)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	return svc.Stop()
}
func main() {
	if e := run(); e != nil {
		log.Fatal(e)
	}
}
