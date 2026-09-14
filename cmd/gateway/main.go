package main

import (
	"context"
	"errors"
	pb "kerthus/gen/go/saas/v1"
	agentcore "kerthus/internal/agent"
	"kerthus/internal/gateway"
	"kerthus/internal/gateway/routing"
	"kerthus/internal/gateway/upload"
	"kerthus/internal/platform/config"
	micro "kerthus/internal/platform/micro"
	"kerthus/internal/platform/storage"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

func run() error {
	c, e := config.Load()
	if e != nil {
		return e
	}
	client, e := micro.Client(c)
	if e != nil {
		return e
	}
	platform := pb.NewPlatformService(micro.ServiceName, client)
	agentStore, e := agentcore.NewMySQLSessionStore(c.DSN)
	if e != nil {
		return e
	}
	defer agentStore.Close()
	store, e := storage.New(c.StorageEndpoint, c.StorageAccessKey, c.StorageSecretKey, c.StorageBucket, c.StorageTLS)
	if e != nil {
		return e
	}
	handler := gateway.New(platform, gateway.Config{AgentStore: agentStore, GatewayKey: c.GatewayKey, StaticURL: c.StaticURL, CORSOrigins: c.CORSOrigins, Upload: upload.HandlerWithConfig(platform, store, c.GatewayKey, upload.Config{AttachmentMaxBytes: c.AttachmentMaxBytes}), Download: upload.Download(platform, store, c.GatewayKey), Files: upload.FilesWithLegacy(store, c.LegacyStaticURL), AppProxy: &routing.Proxy{Platform: platform, Registry: micro.Registry(c), GatewayKey: c.GatewayKey}})
	srv := &http.Server{Addr: c.HTTPAddress, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 2 * time.Minute, WriteTimeout: 2 * time.Minute, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	errch := make(chan error, 1)
	go func() { log.Printf("HTTP gateway listening on %s", c.HTTPAddress); errch <- srv.ListenAndServe() }()
	select {
	case e := <-errch:
		if errors.Is(e, http.ErrServerClosed) {
			return nil
		}
		return e
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdown)
	}
}
func main() {
	if e := run(); e != nil {
		log.Fatal(e)
	}
}
