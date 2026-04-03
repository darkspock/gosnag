.PHONY: dev build run migrate sqlc frontend docker clean admin remote-admin

# Development: run backend + frontend with hot reload
dev:
	@echo "Starting backend..."
	@go run ./cmd/gosnag &
	@cd web && npm run dev

# Build everything
build: frontend
	go build -o gosnag ./cmd/gosnag

# Run the compiled binary
run: build
	./gosnag

# Build frontend only
frontend:
	cd web && npm ci && npm run build

# Run database migrations (requires DATABASE_URL)
migrate:
	go run ./cmd/gosnag migrate

# Regenerate sqlc code from SQL queries
sqlc:
	sqlc generate

# Docker build and run
docker:
	docker compose up --build

# Docker in background
docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

# Add admin user (Docker): make admin EMAIL=user@example.com
admin:
	@test -n "$(EMAIL)" || (echo "Usage: make admin EMAIL=user@example.com" && exit 1)
	docker compose exec db psql -U gosnag -c \
		"INSERT INTO users (id, email, name, role, created_at, updated_at) VALUES (gen_random_uuid(), '$(EMAIL)', '$(EMAIL)', 'admin', now(), now()) ON CONFLICT (email) DO UPDATE SET role = 'admin', updated_at = now();"
	@echo "✓ $(EMAIL) is now admin"

# Add admin user on remote EC2: make remote-admin EMAIL=user@example.com HOST=1.2.3.4
remote-admin:
	@test -n "$(EMAIL)" || (echo "Usage: make remote-admin EMAIL=user@example.com HOST=1.2.3.4" && exit 1)
	@test -n "$(HOST)" || (echo "Usage: make remote-admin EMAIL=user@example.com HOST=1.2.3.4" && exit 1)
	ssh -i ~/.ssh/gosnag-key.pem ec2-user@$(HOST) \
		"sudo docker compose -f /opt/gosnag/docker-compose.yml exec db psql -U gosnag -c \"INSERT INTO users (id, email, name, role, created_at, updated_at) VALUES (gen_random_uuid(), '$(EMAIL)', '$(EMAIL)', 'admin', now(), now()) ON CONFLICT (email) DO UPDATE SET role = 'admin', updated_at = now();\""
	@echo "✓ $(EMAIL) is now admin on $(HOST)"

# Clean build artifacts
clean:
	rm -f gosnag
	rm -rf web/dist web/node_modules
