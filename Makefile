.PHONY: build build-frontend build-backend run dev tui test test-backend test-frontend watch-frontend watch-backend clean

build: build-frontend build-backend

build-frontend:
	cd frontend && npm install && npm run build

build-backend:
	cd backend && go build -o wordgo ./cmd/server

run: build
	cd backend && ./wordgo

dev: build-frontend
	cd backend && go run ./cmd/server

tui:
	cd backend && go run ./cmd/tui

# test runs the fast unit tests. The api package's gameflow tests download
# real dictionaries, so they are excluded here and run on their own:
#   cd backend && go test ./internal/api -run TestPlayDefaultLangs -timeout 2h
test: test-backend test-frontend

test-backend:
	cd backend && go vet ./... && go test ./... -skip 'TestPlay.*'

test-frontend:
	cd frontend && npm install && npm test

watch-frontend:
	cd frontend && npm run watch

watch-backend:
	cd backend && air

clean:
	rm -f backend/wordgo.db
	rm -rf backend/test-logs
	rm -rf backend/tmp
	rm -f frontend/public/script.js
	rm -rf backend/cache
