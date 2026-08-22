.PHONY: dev up down logs seed-demo doctor config-validate migrate migrate-status purge-documents purge-messages backup backup-list backup-verify restore recovery-drill smoke acceptance deploy-preflight deploy-status deploy-apply deploy-rollback backend-test backend-fmt frontend-install frontend-build

HOTELMATE = cd backend && go run ./cmd/hotelmate --

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
	$(HOTELMATE) retention purge-documents --yes

purge-messages:
	$(HOTELMATE) retention purge-messages --yes

migrate:
	$(HOTELMATE) migrate up --yes

migrate-status:
	$(HOTELMATE) migrate status

doctor:
	$(HOTELMATE) doctor

config-validate:
	$(HOTELMATE) config validate

backup:
	$(HOTELMATE) --backup-dir "$(BACKUP_DIR)" backup create --yes

backup-list:
	$(HOTELMATE) --backup-dir "$(BACKUP_DIR)" backup list

backup-verify:
	$(HOTELMATE) --manifest "$(BACKUP_FILE)" backup verify

restore:
	$(HOTELMATE) --manifest "$(BACKUP_FILE)" backup restore $(CONFIRM)

recovery-drill:
	$(HOTELMATE) --config "$(CONFIG_FILE)" backup drill --yes

smoke:
	$(HOTELMATE) --base-url "$(BASE_URL)" smoke

acceptance:
	$(HOTELMATE) --base-url "$(BASE_URL)" acceptance

deploy-preflight:
	$(HOTELMATE) --config "$(CONFIG_FILE)" --release-file "$(RELEASE_FILE)" deploy preflight

deploy-status:
	$(HOTELMATE) --config "$(CONFIG_FILE)" deploy status

deploy-apply:
	$(HOTELMATE) --config "$(CONFIG_FILE)" --release-file "$(RELEASE_FILE)" deploy apply --yes

deploy-rollback:
	$(HOTELMATE) --config "$(CONFIG_FILE)" --release-file "$(RELEASE_FILE)" deploy rollback --yes

backend-test:
	cd backend && go test ./...

backend-fmt:
	cd backend && gofmt -w cmd internal

frontend-install:
	cd frontend && npm install

frontend-build:
	cd frontend && npm run build
