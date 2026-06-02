package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/anhtuanlc/mediahub/internal/config"
	"github.com/anhtuanlc/mediahub/internal/reset"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	confirm := flag.Bool("confirm", false, "bắt buộc để xác nhận xóa toàn bộ dữ liệu")
	force := flag.Bool("force", false, "cho phép chạy khi APP_ENV=production")
	flag.Parse()

	confirmed := *confirm || os.Getenv("RESET_CONFIRM") == "1"
	if !confirmed {
		log.Fatal("Thiếu xác nhận. Chạy: RESET_CONFIRM=1 make reset-data (hoặc: go run ./cmd/reset -confirm)")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	if cfg.AppEnv == "production" && !*force {
		log.Fatal("Không reset khi APP_ENV=production (dùng -force nếu thực sự cần)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	fmt.Println("Đang xóa dữ liệu Postgres, Redis và object storage...")
	res, err := reset.Run(ctx, cfg)
	if err != nil {
		log.Fatalf("reset: %v", err)
	}

	fmt.Println("Reset dữ liệu hoàn tất:")
	fmt.Printf("  - Postgres: đã truncate và seed folder Root\n")
	fmt.Printf("  - Redis: FLUSHALL\n")
	if res.StorageSkipped {
		fmt.Printf("  - Storage: bỏ qua (chưa cấu hình endpoint/bucket)\n")
	} else {
		fmt.Printf("  - Storage: đã xóa %d object trong bucket\n", res.StorageObjects)
	}
	fmt.Println()
	fmt.Println("Hệ thống sẵn sàng thiết lập lại. Mở http://localhost:3000/setup")
	os.Exit(0)
}
