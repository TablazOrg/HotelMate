.PHONY: dev up down logs seed-demo migrate purge-documents purge-messages backup restore smoke backend-test backend-fmt frontend-install frontend-build

dev:
	@echo "Run 'make up' for the full stack, or start the API and frontend separately."

up:
	docker compose up --build

down:
	docker compose down

logs:
	docker compose logs -f api postgres

seed-demo:
	docker compose run --rm --entrypoint /app/seed-demo api

purge-documents:
	docker compose run --rm --entrypoint /app/purge-documents api

purge-messages:
	docker compose run --rm --entrypoint /app/purge-messages api

migrate:
	docker compose run --rm --entrypoint /app/migrate api

backup:
	./scripts/backup.sh "$(BACKUP_DIR)"

restore:
	./scripts/restore.sh "$(BACKUP_FILE)" "$(CONFIRM)"

smoke:
	./scripts/smoke.sh "$(BASE_URL)"

backend-test:
	cd backend && go test ./...

backend-fmt:
	cd backend && gofmt -w cmd internal

frontend-install:
	cd frontend && npm install

frontend-build:
	cd frontend && npm run build
