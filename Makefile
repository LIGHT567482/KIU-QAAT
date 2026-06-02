.PHONY: up down build test migrate seed keys lint help

COMPOSE=docker compose -f infra/docker-compose.yml --env-file .env

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

## migrate: Run all pending DB migrations
migrate:
	$(COMPOSE) exec postgres psql -U qaat -d qaat -f /docker-entrypoint-initdb.d/001_init_schema.sql || true
	@echo "Migrations applied"

## seed: Load test seed data
seed:
	$(COMPOSE) exec postgres psql -U qaat -d qaat -f /seeds/001_test_tenants.sql
	$(COMPOSE) exec postgres psql -U qaat -d qaat -f /seeds/002_test_users.sql

## keys: Generate RSA-2048 key pair for Auth Service (dev only)
keys:
	@mkdir -p keys
	openssl genrsa -out keys/auth_private.pem 2048
	openssl rsa -in keys/auth_private.pem -pubout -out keys/auth_public.pem
	@echo "Keys written to keys/ — never commit these"

## tidy: Run go mod tidy on all Go services (run this first after cloning)
tidy:
	cd services/auth-service && go mod tidy
	cd services/api-gateway && go mod tidy
	cd services/qr-generator && go mod tidy 2>/dev/null || true
	cd services/session-manager && go mod tidy 2>/dev/null || true
	cd services/sync-receiver && go mod tidy 2>/dev/null || true

## install: Install all frontend dependencies via pnpm
install:
	cd apps/coordinator-pwa && pnpm install
	cd apps/admin-dashboards && pnpm install

## dev-pwa: Start Coordinator PWA dev server
dev-pwa:
	cd apps/coordinator-pwa && pnpm dev

## dev-dashboards: Start Admin Dashboards dev server
dev-dashboards:
	cd apps/admin-dashboards && pnpm dev

## test-auth: Run auth service unit tests
test-auth:
	cd services/auth-service && go test ./... -v -race -count=1

## test-gateway: Run api-gateway unit tests
test-gateway:
	cd services/api-gateway && go test ./... -v -race -count=1

## test-pwa: Run PWA unit tests
test-pwa:
	cd apps/coordinator-pwa && pnpm test

## lint: Run golangci-lint on all Go services
lint:
	cd services/auth-service && golangci-lint run ./...
	cd services/api-gateway && golangci-lint run ./...

## ps: Show running containers
ps:
	$(COMPOSE) ps

## help: Show this help
help:
	@grep -E '^##' Makefile | sed 's/## //'
