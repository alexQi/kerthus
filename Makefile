SHELL := /bin/sh
export KERTHUS_ENV_FILE ?= $(CURDIR)/.local/saas.env
COMPOSE = docker compose --env-file .local/saas.env -f deploy/compose.yaml
.PHONY: init infra migrate seed build deps dev saas gateway web test integration smoke smoke-upload smoke-crud smoke-grants smoke-approval smoke-dialogs smoke-session check generate tools stop-infra
init:
	python3 scripts/init-dev.py
infra: init
	$(COMPOSE) up -d --wait
migrate:
	go run ./cmd/saasctl migrate
seed:
	go run ./cmd/saasctl seed
build:
	go build -o bin/saas ./cmd/saas
	go build -o bin/gateway ./cmd/gateway
	go build -o bin/saasctl ./cmd/saasctl
deps:
	go mod download
	cd web/admin && yarn install --frozen-lockfile --ignore-scripts
dev: build
	python3 scripts/dev.py
saas:
	go run ./cmd/saas
gateway:
	go run ./cmd/gateway
web:
	cd web/admin && yarn dev
test:
	go test ./...
	go vet ./...
	python3 scripts/check-boundaries.py
integration:
	python3 scripts/test-integration.py
smoke:
	python3 scripts/smoke-api.py
smoke-upload:
	python3 scripts/smoke-upload.py
smoke-crud:
	python3 scripts/test-browser-crud.py
smoke-grants:
	python3 scripts/test-browser-crud.py scripts/smoke-grants.cjs
smoke-approval:
	python3 scripts/test-browser-crud.py scripts/smoke-approval.cjs
smoke-dialogs:
	python3 scripts/test-browser-crud.py scripts/smoke-dialogs.cjs
smoke-session:
	node scripts/smoke-session.cjs
check: test
	cd web/admin && yarn type:check && yarn build
tools:
	GOBIN=$(CURDIR)/.tools go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
	GOBIN=$(CURDIR)/.tools go install go-micro.dev/v6/cmd/protoc-gen-micro@v6.13.0
generate:
	PATH="$(CURDIR)/.tools:$$PATH" protoc -I api/proto --go_out=. --go_opt=module=kerthus --micro_out=paths=source_relative:gen/go api/proto/saas/v1/core.proto
stop-infra:
	$(COMPOSE) stop
