package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/anhtuanlc/mediahub/internal/config"
	"github.com/anhtuanlc/mediahub/internal/platform"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/service"
	"github.com/anhtuanlc/mediahub/internal/storage"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	apply := flag.Bool("apply", false, "xóa orphan thật (mặc định dry-run)")
	maxDelete := flag.Int("max", service.MaxOrphanDeletesPerRun, "số object tối đa xóa mỗi lần")
	var prefixes flagPrefixes
	flag.Var(&prefixes, "prefix", "prefix S3 quét (lặp được; mặc định thumbnails/, hls/, originals/)")
	flag.Parse()

	dryRun := !*apply
	if !dryRun && os.Getenv("ORPHAN_CLEANUP_CONFIRM") != "1" {
		log.Fatal("Xóa orphan cần xác nhận: ORPHAN_CLEANUP_CONFIRM=1 go run ./cmd/orphan-cleanup -apply")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	pool, err := platform.NewPostgresPool(ctx, cfg.DBDSN)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer pool.Close()

	store, err := storage.NewS3Storage(ctx, cfg.Storage, 0)
	if err != nil {
		log.Fatalf("storage: %v", err)
	}

	objects := repository.NewMediaObjectRepository(pool)
	deletionJobs := repository.NewStorageDeletionRepository(pool)
	uploadSessions := repository.NewUploadSessionRepository(pool)
	cleanup := service.NewStorageCleanupService(deletionJobs, store)
	maintenance := service.NewMaintenanceService(cleanup, nil, objects, deletionJobs, uploadSessions, store, nil)

	in := service.CleanupOrphansInput{
		DryRun:    dryRun,
		MaxDelete: *maxDelete,
		Prefixes:  prefixes,
	}

	fmt.Printf("Quét orphan (dry_run=%v)...\n", dryRun)
	res, err := maintenance.CleanupOrphans(ctx, in)
	if err != nil {
		log.Fatalf("cleanup orphans: %v", err)
	}

	fmt.Printf("  scanned:      %d\n", res.Scanned)
	fmt.Printf("  orphan_count: %d\n", res.OrphanCount)
	fmt.Printf("  deleted:      %d\n", res.Deleted)
	if res.Truncated {
		fmt.Println("  (truncated — chạy lại để xóa tiếp)")
	}
	if len(res.SampleKeys) > 0 {
		fmt.Println("  sample keys:")
		for _, k := range res.SampleKeys {
			fmt.Printf("    - %s\n", k)
		}
	}
	if dryRun && res.OrphanCount > 0 {
		fmt.Println()
		fmt.Println("Dry-run — không xóa. Áp dụng: ORPHAN_CLEANUP_CONFIRM=1 go run ./cmd/orphan-cleanup -apply")
	}
}

type flagPrefixes []string

func (f *flagPrefixes) String() string {
	return strings.Join(*f, ",")
}

func (f *flagPrefixes) Set(v string) error {
	*f = append(*f, v)
	return nil
}
