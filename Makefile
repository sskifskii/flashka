DATABASE_URL ?= postgres://flashka:flashka@localhost:5432/flashka?sslmode=disable
export DATABASE_URL

.PHONY: dev web-build db-up db-down migrate-up migrate-down migrate-check fmt fmt-check lint test test-integration test-e2e build openapi-check secret-check check
dev: db-up
	$(MAKE) migrate-up
	$(MAKE) web-build
	go run ./cmd/server
web/node_modules/.flashka-install: web/package.json web/package-lock.json
	cd web && npm ci
	touch $@
web-build: web/node_modules/.flashka-install
	cd web && npm run build
db-up:
	docker compose up -d --wait postgres
db-down:
	docker compose down
migrate-up:
	go run ./cmd/migrate up
migrate-down:
	go run ./cmd/migrate down
migrate-check:
	./scripts/migration-check.sh
fmt:
	gofmt -w $$(find cmd internal -name '*.go')
	cd web && npm run format
fmt-check:
	test -z "$$(gofmt -l cmd internal)"
	cd web && npm run format:check
lint:
	go vet ./cmd/... ./internal/...
	cd web && npm run lint && npm run typecheck
test:
	go test -race ./cmd/... ./internal/...
	cd web && npm test
test-integration:
	go test -race -p 1 -tags=integration ./internal/...
test-e2e: build
	cd web && npm run e2e
build:
	cd web && npm run build
	go build -o bin/flashka ./cmd/server
openapi-check:
	npx --yes @redocly/cli@1.34.5 lint api/openapi.yaml
secret-check:
	./scripts/secret-check.sh
check: fmt-check lint test openapi-check secret-check build
