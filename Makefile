.PHONY: build build-frontend build-backend run dev watch-frontend watch-backend clean

build: build-frontend build-backend

build-frontend:
	cd frontend && npm install && npm run build

build-backend:
	cd backend && go build -o wordgo ./cmd/server

run: build
	cd backend && ./wordgo

dev: build-frontend
	cd backend && go run ./cmd/server

watch-frontend:
	cd frontend && npm run watch

watch-backend:
	cd backend && air

clean:
	rm -f backend/wordgo.db
	rm -rf backend/tmp
	rm -f frontend/public/script.js
	rm -rf backend/cache
