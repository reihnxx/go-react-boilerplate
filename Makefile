# Run `make help` for the list of targets. Backend targets read backend/.env.
SHELL := /bin/bash
BACKEND := backend
TEST_DATABASE_URL ?= postgres://postgres@127.0.0.1:5432/go_boilerplate_test?sslmode=disable

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'

# ---------- backend ----------

.PHONY: dev-api
dev-api: ## Run the API (go run) on HTTP_ADDR
	cd $(BACKEND) && go run ./cmd/api serve

.PHONY: build-api
build-api: ## Build the API binary into backend/bin/api
	cd $(BACKEND) && CGO_ENABLED=0 go build -trimpath -o bin/api ./cmd/api

.PHONY: migrate
migrate: ## Apply pending migrations
	cd $(BACKEND) && go run ./cmd/api migrate up

.PHONY: migrate-down
migrate-down: ## Roll back the last migration
	cd $(BACKEND) && go run ./cmd/api migrate down

.PHONY: migrate-status
migrate-status: ## Show migration status
	cd $(BACKEND) && go run ./cmd/api migrate status

.PHONY: create-admin
create-admin: ## Create the first super-admin: make create-admin EMAIL=… [USERNAME=admin] [NAME="Super Admin"]
	@test -n "$(EMAIL)" || (echo "EMAIL is required" && exit 1)
	cd $(BACKEND) && go run ./cmd/api create-admin -email "$(EMAIL)" -username "$(or $(USERNAME),admin)" -name "$(or $(NAME),Super Admin)"

.PHONY: seed
seed: ## Create development sample users (refuses in production)
	cd $(BACKEND) && go run ./cmd/api seed

.PHONY: test-api
test-api: ## Backend unit tests (no database needed)
	cd $(BACKEND) && go test ./...

.PHONY: test-api-integration
test-api-integration: ## Backend unit + integration tests against TEST_DATABASE_URL
	cd $(BACKEND) && TEST_DATABASE_URL='$(TEST_DATABASE_URL)' go test -race -tags integration -count=1 ./...

.PHONY: lint-api
lint-api: ## gofmt check + go vet (both build tags)
	@cd $(BACKEND) && unformatted=$$(gofmt -l .); if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi
	cd $(BACKEND) && go vet ./... && go vet -tags integration ./...

.PHONY: check-api
check-api: lint-api build-api test-api-integration ## All backend checks

# ---------- frontend ----------
WEB := frontend

.PHONY: install-web
install-web: ## Install frontend dependencies (npm ci)
	cd $(WEB) && npm ci

.PHONY: dev-web
dev-web: ## Run the Vite dev server on :5173 (proxies /api to the API)
	cd $(WEB) && npm run dev

.PHONY: build-web
build-web: ## Type-check and build the SPA into frontend/dist
	cd $(WEB) && npm run build

.PHONY: lint-web
lint-web: ## ESLint + TypeScript type check + Prettier check
	cd $(WEB) && npm run lint && npm run typecheck && npm run format:check

.PHONY: test-web
test-web: ## Frontend unit/component tests (Vitest)
	cd $(WEB) && npm test

.PHONY: check-web
check-web: lint-web test-web build-web ## All frontend checks

.PHONY: check
check: check-api check-web ## Everything: backend + frontend checks

# ---------- docker compose (full stack) ----------

.PHONY: up
up: ## Build and start db + migrate + api + web (needs ./.env, see .env.example)
	docker compose up -d --build

.PHONY: down
down: ## Stop the stack (keeps the database volume)
	docker compose down

.PHONY: logs
logs: ## Follow logs of all services
	docker compose logs -f

.PHONY: ps
ps: ## Show service status
	docker compose ps

.PHONY: up-admin
up-admin: ## Create a super-admin in the compose stack: ADMIN_PASSWORD=… make up-admin EMAIL=…
	@test -n "$(EMAIL)" || (echo "EMAIL is required" && exit 1)
	docker compose run --rm -e ADMIN_PASSWORD api create-admin -email "$(EMAIL)" -username "$(or $(USERNAME),admin)" -name "$(or $(NAME),Super Admin)"

.PHONY: reset-db
reset-db: ## DANGER: stop the stack and delete the database volume
	docker compose down -v
