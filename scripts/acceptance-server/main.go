// acceptance-server runs the real gateway/RPC/core against a disposable database.
// It never registers in the development Consul or uses the development dataset.
package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	driver "github.com/go-sql-driver/mysql"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go-micro.dev/v6/client"
	cg "go-micro.dev/v6/client/grpc"
	"go-micro.dev/v6/registry"
	"go-micro.dev/v6/selector"
	"go-micro.dev/v6/server"
	sg "go-micro.dev/v6/server/grpc"
	pb "kerthus/gen/go/saas/v1"
	"kerthus/internal/gateway"
	"kerthus/internal/gateway/upload"
	"kerthus/internal/platform/appmanifest"
	"kerthus/internal/platform/config"
	"kerthus/internal/platform/rpcauth"
	"kerthus/internal/platform/storage"
	"kerthus/internal/saas/adapters/mysql"
	"kerthus/internal/saas/adapters/redis"
	rpc "kerthus/internal/saas/adapters/rpc"
	"kerthus/internal/saas/domain/dictionary"
	"kerthus/internal/saas/domain/identity"
	"kerthus/internal/saas/ports"
	"kerthus/internal/saas/usecase/core"
)

// Track only sessions created by this test process, for exact cleanup.
type sessions struct {
	*redis.Sessions
	mu     sync.Mutex
	tokens []string
}

func (s *sessions) Put(ctx context.Context, token string, value identity.Session, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens = append(s.tokens, token)
	return s.Sessions.Put(ctx, token, value, ttl)
}
func (s *sessions) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, token := range s.tokens {
		_ = s.Delete(context.Background(), token)
	}
}

func random() string {
	var b [24]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b[:])
}

func run() error {
	c, e := config.Load()
	if e != nil {
		return e
	}
	testDSN := os.Getenv("KERTHUS_TEST_MYSQL_DSN")
	if testDSN == "" {
		return fmt.Errorf("explicit KERTHUS_TEST_MYSQL_DSN required")
	}
	cfg, e := driver.ParseDSN(testDSN)
	if e != nil {
		return fmt.Errorf("invalid test DSN")
	}
	cfg.DBName = ""
	admin, e := sql.Open("mysql", cfg.FormatDSN())
	if e != nil {
		return e
	}
	defer admin.Close()
	name := "kerthus_test_browser_" + random()[:12]
	if _, e = admin.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); e != nil {
		return e
	}
	defer func() {
		if _, err := admin.Exec("DROP DATABASE `" + name + "`"); err != nil {
			fmt.Fprintln(os.Stderr, "test database cleanup failed:", err)
		}
	}()
	cfg.DBName, cfg.ParseTime, cfg.ClientFoundRows = name, true, true
	db, e := mysql.Open(cfg.FormatDSN())
	if e != nil {
		return e
	}
	defer db.Close()
	ctx := context.Background()
	if e = db.Migrate(ctx); e != nil {
		return e
	}
	rs := redis.NewNamespaced(c.RedisAddress, c.RedisPassword, name)
	defer rs.Close()
	tracked := &sessions{Sessions: rs}
	defer tracked.cleanup()
	var objects ports.Objects
	var objectStore *storage.Store
	if os.Getenv("KERTHUS_ACCEPTANCE_STORAGE") == "true" {
		bucket := "kerthus-visual-" + random()[:12]
		objectStore, e = storage.New(c.StorageEndpoint, c.StorageAccessKey, c.StorageSecretKey, bucket, c.StorageTLS)
		if e != nil {
			return e
		}
		if e = objectStore.EnsureBucket(ctx); e != nil {
			return e
		}
		cleanupClient, err := minio.New(c.StorageEndpoint, &minio.Options{Creds: credentials.NewStaticV4(c.StorageAccessKey, c.StorageSecretKey, ""), Secure: c.StorageTLS})
		if err != nil {
			return err
		}
		defer func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			for object := range cleanupClient.ListObjects(cleanupCtx, bucket, minio.ListObjectsOptions{Recursive: true}) {
				if object.Err != nil {
					fmt.Fprintln(os.Stderr, "test object listing cleanup failed:", object.Err)
					break
				}
				if err := cleanupClient.RemoveObject(cleanupCtx, bucket, object.Key, minio.RemoveObjectOptions{}); err != nil {
					fmt.Fprintln(os.Stderr, "test object cleanup failed:", err)
				}
			}
			if err := cleanupClient.RemoveBucket(cleanupCtx, bucket); err != nil {
				fmt.Fprintln(os.Stderr, "test bucket cleanup failed:", err)
			}
		}()
		objects = objectStore
	}
	svc := core.New(db, tracked, objects)
	for _, code := range []string{"system", "basic"} {
		manifest, err := appmanifest.Load(".", filepath.Join("applications", code, "app.yaml"))
		if err != nil {
			return err
		}
		if err = svc.RegisterApplication(ctx, manifest.Definition()); err != nil {
			return err
		}
	}
	phone, password := "13900009999", random()
	// Manual visual acceptance uses an explicitly synthetic, documented password.
	if fixturePassword := os.Getenv("KERTHUS_ACCEPTANCE_PASSWORD"); fixturePassword != "" {
		password = fixturePassword
	}
	if e = svc.Bootstrap(ctx, phone, password); e != nil {
		return e
	}
	if e = svc.ImportDistricts(ctx, []dictionary.District{
		{ID: 110000, Name: "验收省"}, {ID: 110100, ParentID: 110000, Name: "验收市"},
		{ID: 110101, ParentID: 110100, Name: "验收区"},
	}); e != nil {
		return e
	}
	reg, key := registry.NewMemoryRegistry(), random()
	rpcServer := sg.NewServer(server.Name("kerthus.acceptance"), server.Address("127.0.0.1:0"), server.Registry(reg))
	h, e := rpc.New(svc, rpcauth.Config{GatewayKey: key})
	if e != nil {
		return e
	}
	if e = pb.RegisterPlatformHandler(rpcServer, h); e != nil {
		return e
	}
	if e = rpcServer.Start(); e != nil {
		return e
	}
	defer rpcServer.Stop()
	cli := cg.NewClient(client.Registry(reg), client.Selector(selector.NewSelector(selector.Registry(reg))), client.RequestTimeout(10*time.Second), client.Retries(0))
	platform := pb.NewPlatformService("kerthus.acceptance", cli)
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return e
	}
	defer listener.Close()
	adminURL := os.Getenv("KERTHUS_ADMIN_URL")
	if adminURL == "" {
		adminURL = "http://127.0.0.1:15173"
	}
	gatewayConfig := gateway.Config{GatewayKey: key, CORSOrigins: []string{adminURL}}
	if objectStore != nil {
		gatewayConfig.StaticURL = "http://" + listener.Addr().String() + "/files"
		gatewayConfig.Upload = upload.Handler(platform, objectStore, key)
		gatewayConfig.Download = upload.Download(platform, objectStore, key)
		gatewayConfig.Files = upload.Files(objectStore)
	}
	httpServer := &http.Server{Handler: gateway.New(platform, gatewayConfig), ReadHeaderTimeout: 5 * time.Second}
	defer httpServer.Close()
	errch := make(chan error, 1)
	go func() { errch <- httpServer.Serve(listener) }()
	result, _ := json.Marshal(map[string]string{"base": "http://" + listener.Addr().String(), "phone": phone, "password": password})
	path := os.Getenv("KERTHUS_ACCEPTANCE_CONFIG")
	if path == "" {
		return fmt.Errorf("KERTHUS_ACCEPTANCE_CONFIG is required")
	}
	if e = os.WriteFile(path, result, 0600); e != nil {
		return e
	}
	defer os.Remove(path)
	fmt.Println("Isolated browser acceptance server ready")
	stopCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	select {
	case <-stopCtx.Done():
		return nil
	case e = <-errch:
		return e
	}
}

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
