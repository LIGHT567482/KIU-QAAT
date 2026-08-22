.PHONY: up down build test migrate migrate-status migrate-adopt seed keys tidy install lint help contabo-up contabo-down contabo-logs

COMPOSE=docker-compose -f infra/docker-compose.yml --env-file .env
COMPOSE_PROD=docker compose -f infra/docker-compose.prod.yml --env-file .env.production

# Migrations run as the OWNER role (they create tables, policies and roles), so this is the
# admin URL, not the RLS-confined qaat_app one. Override for a remote database:
#   make migrate MIGRATE_DB_URL="postgres://…@…render.com/qaat?sslmode=require"
MIGRATE_DB_URL ?= postgres://qaat:$(or $(DB_PASSWORD),changeme_db)@localhost:5434/qaat?sslmode=disable

## up: Start all services (dev)
up:
	$(COMPOSE) up -d

## down: Stop all services
down:
	$(COMPOSE) down

## build: Build all service images
build:
	$(COMPOSE) build

## logs: Tail all service logs
logs:
	$(COMPOSE) logs -f

## migrate: Apply every pending DB migration (ledger-tracked, safe to re-run)
migrate:
	cd backend/api-gateway && go run ./cmd/migrate -db "$(MIGRATE_DB_URL)" up

## migrate-status: Show which migrations are applied and which are pending
migrate-status:
	cd backend/api-gateway && go run ./cmd/migrate -db "$(MIGRATE_DB_URL)" status

## migrate-adopt: FIRST run against a database migrated by hand (steps over what already exists)
migrate-adopt:
	cd backend/api-gateway && go run ./cmd/migrate -db "$(MIGRATE_DB_URL)" up --adopt

## seed: Load test seed data
seed:
	@echo "Seeding is retired. QAAT serves ONE institution — Kampala International University —"
	@echo "and the old seeds created Alpha/Beta/Test tenants beside it. Those files are kept as"
	@echo "*.sql.retired for reference only; running them would put the multi-tenant demo data"
	@echo "back into a database that is now single-institution by design."
	@echo ""
	@echo "To add accounts, use Administration -> Import in the dashboard."
	@exit 1

## keys: Generate RSA-2048 key pair for Auth Service (dev only)
keys:
	@mkdir -p keys
	openssl genrsa -out keys/auth_private.pem 2048
	openssl rsa -in keys/auth_private.pem -pubout -out keys/auth_public.pem
	@echo "Keys written to keys/ — never commit these"

## tidy: Run go mod tidy on all Go services (run this first after cloning)
tidy:
	cd backend/auth-service && go mod tidy
	cd backend/api-gateway && go mod tidy
	cd backend/session-manager && go mod tidy 2>/dev/null || true
	cd backend/sync-receiver && go mod tidy 2>/dev/null || true

## install: Install all frontend dependencies via pnpm
install:
	cd frontend/admin-dashboards && pnpm install
	cd frontend/student-portal && pnpm install

## dev-dashboards: Start Admin Dashboards dev server
dev-dashboards:
	cd frontend/admin-dashboards && pnpm dev

## test-auth: Run auth service unit tests
test-auth:
	cd backend/auth-service && go test ./... -v -race -count=1

## test-gateway: Run api-gateway unit tests
test-gateway:
	cd backend/api-gateway && go test ./... -v -race -count=1

## test-dashboards: Typecheck + unit-test the admin dashboards
test-dashboards:
	cd frontend/admin-dashboards && pnpm typecheck && pnpm test

## lint: Run golangci-lint on all Go services
lint:
	cd backend/auth-service && golangci-lint run ./...
	cd backend/api-gateway && golangci-lint run ./...

## ps: Show running containers
ps:
	$(COMPOSE) ps

## contabo-up: Build and start the Contabo production stack (needs .env.production)
contabo-up:
	bash scripts/contabo/deploy-web-on-server.sh

## contabo-down: Stop the Contabo production stack (keeps volumes)
contabo-down:
	$(COMPOSE_PROD) down

## contabo-logs: Tail Contabo production logs
contabo-logs:
	$(COMPOSE_PROD) logs -f

## help: Show this help
help:
	@grep -E '^##' Makefile | sed 's/## //'
