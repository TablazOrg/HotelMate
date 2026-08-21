.PHONY: dev up down logs backend-test backend-fmt frontend-install frontend-build

dev:
	@echo "Run 'make up' for the full stack, or start the API and frontend separately."

up:
	docker compose up --build

down:
	docker compose down

logs:
	docker compose logs -f api postgres

backend-test:
	cd backend && go test ./...

backend-fmt:
	cd backend && gofmt -w cmd internal

frontend-install:
	cd frontend && npm install

frontend-build:
	cd frontend && npm run build
