# Riha: common tasks. Requires Go 1.24+, Node 20+ and Postgres 16.
DATABASE_URL ?= postgres://riha:riha@localhost:5432/riha?sslmode=disable

.PHONY: db api web build run test reset

db:            ## start Postgres in Docker
	docker compose up -d db

api:           ## run the Go API on :8080 (dev, with demo seed)
	cd backend && DATABASE_URL=$(DATABASE_URL) RIHA_ENV=development RIHA_SEED=true go run ./cmd/server

web:           ## run the Vite dev server on :5173 (proxies /api and /v1 to :8080)
	cd frontend && npm install && npm run dev

build:         ## build the frontend and the API binary
	cd frontend && npm ci && npm run build
	cd backend && go build -o bin/riha ./cmd/server

run: build     ## single process: API + built frontend on :8080
	cd backend && DATABASE_URL=$(DATABASE_URL) RIHA_ENV=development RIHA_SEED=true RIHA_STATIC_DIR=../frontend/dist RIHA_PUBLIC_URL=http://localhost:8080 ./bin/riha

test:          ## Go tests (service tests need RIHA_TEST_DATABASE_URL)
	cd backend && go test ./...
	cd frontend && npx tsc -p tsconfig.app.json --noEmit

reset:         ## drop and recreate all tables (re-seeds on next start)
	psql "$(DATABASE_URL)" -c 'DROP SCHEMA public CASCADE; CREATE SCHEMA public;'
