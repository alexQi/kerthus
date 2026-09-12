package main

import (
	"context"
	"fmt"
	"kerthus/internal/platform/appmanifest"
	"kerthus/internal/platform/config"
	"kerthus/internal/platform/districtimport"
	"kerthus/internal/platform/legacy"
	"kerthus/internal/platform/storage"
	"kerthus/internal/saas/adapters/mysql"
	"kerthus/internal/saas/usecase/core"
	"os"
	"path/filepath"
	"time"
)

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: saasctl migrate | seed | register <manifest-path> | validate <manifest-path> | import-districts <csv-path>")
	}
	cmd := os.Args[1]
	root, e := os.Getwd()
	if e != nil {
		return e
	}
	if cmd == "validate" {
		if len(os.Args) != 3 {
			return fmt.Errorf("manifest path is required")
		}
		m, e := appmanifest.Load(root, os.Args[2])
		if e != nil {
			return e
		}
		fmt.Printf("valid: %s v%d (%d resources, %d operations)\n", m.AppCode, m.ResourceVersion, len(m.Resources), len(m.Operations))
		return nil
	}
	if cmd == "import-legacy" {
		if len(os.Args) != 4 {
			return fmt.Errorf("usage: saasctl import-legacy <source-env-file> <new-kerthus-database>")
		}
		if e := config.LoadEnv(os.Args[2]); e != nil {
			return e
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		report, e := legacy.Run(ctx, legacy.Options{SourceDSN: os.Getenv("KERTHUS_LEGACY_MYSQL_DSN"), TargetDatabase: os.Args[3], ProjectRoot: root, ReportPath: filepath.Join(root, ".local", os.Args[3]+"-import-report.json")})
		if e != nil {
			return e
		}
		fmt.Printf("Imported %d users and %d tenants into %s; report saved locally.\n", report.Imported["users"], report.Imported["tenants"], report.Target)
		return nil
	}
	c, e := config.Load()
	if e != nil {
		return e
	}
	db, e := mysql.Open(c.DSN)
	if e != nil {
		return e
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	svc := core.New(db, nil, nil)
	switch cmd {
	case "migrate":
		return db.Migrate(ctx)
	case "import-districts":
		if len(os.Args) != 3 {
			return fmt.Errorf("CSV export path is required")
		}
		f, e := os.Open(os.Args[2])
		if e != nil {
			return e
		}
		defer f.Close()
		rows, e := districtimport.Read(f)
		if e != nil {
			return e
		}
		return svc.ImportDistricts(ctx, rows)
	case "register":
		if len(os.Args) != 3 {
			return fmt.Errorf("manifest path is required")
		}
		m, e := appmanifest.Load(root, os.Args[2])
		if e != nil {
			return e
		}
		return svc.RegisterApplication(ctx, m.Definition())
	case "seed":
		for _, code := range []string{"system", "basic"} {
			m, e := appmanifest.Load(root, filepath.Join("applications", code, "app.yaml"))
			if e != nil {
				return e
			}
			if e = svc.RegisterApplication(ctx, m.Definition()); e != nil {
				return e
			}
		}
		if e = svc.Bootstrap(ctx, c.AdminPhone, c.AdminPassword); e != nil {
			return e
		}
		store, e := storage.New(c.StorageEndpoint, c.StorageAccessKey, c.StorageSecretKey, c.StorageBucket, c.StorageTLS)
		if e != nil {
			return e
		}
		return store.EnsureBucket(ctx)
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	fmt.Println("done")
}
