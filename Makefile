.PHONY: infra-up infra-down infra-reset migrate-up migrate-down reset reset-data api scheduler worker fe dev orphan-cleanup orphan-cleanup-apply

COMPOSE_FILE := deployments/docker-compose.yml
COMPOSE_ENV  := --env-file deployments/.env
infra-up:
	docker compose -f $(COMPOSE_FILE) $(COMPOSE_ENV) up -d

infra-down:
	docker compose -f $(COMPOSE_FILE) $(COMPOSE_ENV) down

infra-reset:
	docker compose -f $(COMPOSE_FILE) $(COMPOSE_ENV) down -v
	docker compose -f $(COMPOSE_FILE) $(COMPOSE_ENV) up -d
	@echo "Đợi Postgres sẵn sàng (~10s) rồi chạy: make migrate-up"

migrate-up:
	cd backend && go run ./cmd/migrate up

migrate-down:
	cd backend && go run ./cmd/migrate down

# Xóa volume Docker + migrate lại — hệ thống như mới cài đặt.
# Yêu cầu: RESET_CONFIRM=1 make reset
reset:
	@if [ "$(RESET_CONFIRM)" != "1" ]; then \
		echo "Cảnh báo: lệnh này xóa toàn bộ Postgres, MinIO và Redis (volume Docker)."; \
		echo "Chạy: RESET_CONFIRM=1 make reset"; \
		exit 1; \
	fi
	docker compose -f $(COMPOSE_FILE) $(COMPOSE_ENV) down -v
	docker compose -f $(COMPOSE_FILE) $(COMPOSE_ENV) up -d
	@chmod +x scripts/wait-postgres.sh
	@./scripts/wait-postgres.sh
	$(MAKE) migrate-up
	@echo ""
	@echo "Reset hoàn tất. Tạo Owner mới tại http://localhost:3000/setup"

# Xóa dữ liệu ứng dụng (giữ volume Docker): truncate DB, FLUSHALL Redis, xóa object MinIO.
# Yêu cầu: RESET_CONFIRM=1 make reset-data
reset-data:
	@if [ "$(RESET_CONFIRM)" != "1" ]; then \
		echo "Cảnh báo: xóa users, media, sessions Redis và file trên MinIO."; \
		echo "Chạy: RESET_CONFIRM=1 make reset-data"; \
		exit 1; \
	fi
	cd backend && go run ./cmd/reset -confirm

api:
	cd backend && go run ./cmd/api

# Background jobs: upload expiry, storage deletions, trash purge, temp cleanup, closure repair.
scheduler:
	cd backend && go run ./cmd/scheduler

# Video convert queue (asynq) — tạm chưa dùng khi chưa bật Video Manager.
worker:
	cd backend && go run ./cmd/worker

# Dry-run quét blob MinIO không còn tham chiếu DB.
# Áp dụng: ORPHAN_CLEANUP_CONFIRM=1 make orphan-cleanup-apply
orphan-cleanup:
	cd backend && go run ./cmd/orphan-cleanup

orphan-cleanup-apply:
	cd backend && ORPHAN_CLEANUP_CONFIRM=1 go run ./cmd/orphan-cleanup -apply

fe:
	cd frontend && pnpm dev

dev: infra-up migrate-up
	@echo "Chạy API: make api | Scheduler: make scheduler | FE: make fe"
