.PHONY: infra-up infra-down infra-reset migrate-up migrate-down reset reset-data gateway gateaway api scheduler worker fe dev dev-gateway dev-full orphan-cleanup orphan-cleanup-apply test test-backend docs-sync deploy-prod

# Official Go install often lives here; Cursor/SSH shells may not have reloaded ~/.bashrc yet.
export PATH := /usr/local/go/bin:$(PATH)
GO ?= go

test: test-backend

test-backend:
	cd backend && $(GO) test ./... -count=1 -short

COMPOSE_FILE := deployments/docker-compose.yml
COMPOSE_OVERRIDE := deployments/docker-compose.override.yml
# -f disables automatic override loading; include it explicitly when present (e.g. MinIO port remap).
COMPOSE_FILES := -f $(COMPOSE_FILE) $(if $(wildcard $(COMPOSE_OVERRIDE)),-f $(COMPOSE_OVERRIDE))
COMPOSE_ENV  := --env-file deployments/.env
infra-up:
	docker compose $(COMPOSE_FILES) $(COMPOSE_ENV) up -d

infra-down:
	docker compose $(COMPOSE_FILES) $(COMPOSE_ENV) down

infra-reset:
	docker compose $(COMPOSE_FILES) $(COMPOSE_ENV) down -v
	docker compose $(COMPOSE_FILES) $(COMPOSE_ENV) up -d
	@echo "Đợi Postgres sẵn sàng (~10s) rồi chạy: make migrate-up"

migrate-up:
	cd backend && $(GO) run ./cmd/migrate up

migrate-down:
	cd backend && $(GO) run ./cmd/migrate down

# Xóa volume Docker + migrate lại — hệ thống như mới cài đặt.
# Yêu cầu: RESET_CONFIRM=1 make reset
reset:
	@if [ "$(RESET_CONFIRM)" != "1" ]; then \
		echo "Cảnh báo: lệnh này xóa toàn bộ Postgres, MinIO và Redis (volume Docker)."; \
		echo "Chạy: RESET_CONFIRM=1 make reset"; \
		exit 1; \
	fi
	docker compose $(COMPOSE_FILES) $(COMPOSE_ENV) down -v
	docker compose $(COMPOSE_FILES) $(COMPOSE_ENV) up -d
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
	cd backend && $(GO) run ./cmd/reset -confirm

# Entry point backend (dev + production): kiểm tra môi trường + API + scheduler + worker.
gateway:
	cd backend && $(GO) run ./cmd/gateway

# Alias lỗi chính tả phổ biến
gateaway: gateway

# Chạy riêng từng thành phần (debug).
api:
	cd backend && $(GO) run ./cmd/api

# Background jobs: upload expiry, storage deletions, trash purge, temp cleanup, closure repair.
scheduler:
	cd backend && $(GO) run ./cmd/scheduler

# Video convert queue (Asynq + FFmpeg) — cần khi dùng /videos.
worker:
	cd backend && $(GO) run ./cmd/worker

# Dry-run quét blob MinIO không còn tham chiếu DB.
# Áp dụng: ORPHAN_CLEANUP_CONFIRM=1 make orphan-cleanup-apply
orphan-cleanup:
	cd backend && $(GO) run ./cmd/orphan-cleanup

orphan-cleanup-apply:
	cd backend && ORPHAN_CLEANUP_CONFIRM=1 $(GO) run ./cmd/orphan-cleanup -apply

fe:
	cd frontend && pnpm dev

# Gateway backend: kiểm tra môi trường + API + scheduler + worker (một terminal).
# Frontend chạy riêng: make fe
dev-gateway:
	@chmod +x scripts/dev-gateway.sh
	@exec ./scripts/dev-gateway.sh

# Dừng stack dev sót sau Ctrl+C (go-build binary, go run, next dev).
dev-stop:
	@chmod +x scripts/dev-stop.sh
	@./scripts/dev-stop.sh

# Infra + migrate + gateway backend (khởi động nhanh stack dev).
dev-full: infra-up migrate-up dev-gateway

dev: infra-up migrate-up
	@echo "Backend: make gateway  (hoặc make dev-gateway)"
	@echo "Frontend (terminal khác): make fe"
	@echo "Debug từng thành phần: make api | make scheduler | make worker"

# Build + đồng bộ runtime production ra thư mục riêng (mặc định ~/mediahub).
# Override: MEDIAHUB_DEPLOY_ROOT=/path make deploy-prod
deploy-prod:
	@chmod +x scripts/deploy-prod.sh
	@./scripts/deploy-prod.sh

# Đồng bộ docs/ → frontend/public/docs/ (markdown + OpenAPI + Postman).
docs-sync:
	@mkdir -p frontend/public/docs
	rsync -a --delete \
		--include='README.md' \
		--include='getting-started/***' \
		--include='configuration/***' \
		--include='integration/***' \
		--include='usage/***' \
		--include='reference/***' \
		--include='api/' \
		--include='api/**' \
		--exclude='*' \
		docs/ frontend/public/docs/
	cp docs/api/openapi.yaml frontend/public/docs/openapi.yaml
	cp docs/api/postman-collection.json frontend/public/docs/postman-collection.json
	@rm -f frontend/public/docs/integration.md
	@echo "docs-sync: frontend/public/docs/ updated"
