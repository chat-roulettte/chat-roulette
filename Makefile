DATABASE=chat-roulette
MIGRATIONS_DIR=internal/database/migrations
MIGRATIONS_SOURCE ?= file iofs


go/install:
	go get -v

go/tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.60.1
	go install -tags 'postgres $(MIGRATIONS_SOURCE)' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.15.2
	go install gotest.tools/gotestsum@v1.12.0
	go install github.com/miniscruff/changie@v1.19.1

go/tidy:
	go mod tidy -compat=1.22

go/test:
	go test -v --cover ./...

go/testsum:
	gotestsum --format testname --no-color=false -- --cover ./...

go/lint:
	golangci-lint run --allow-parallel-runners

go/run:
	./scripts/go.sh run

go/build:
	./scripts/go.sh build

go/coverage:
	go tool cover -html=.coverage.out

go/clean:
	go clean -modcache
	rm -rf .coverage.out bin/

generate/key:
	openssl rand -hex 32

docker/build/app-manifest:
	docker build --platform linux/amd64 -t ghcr.io/chat-roulettte/app-manifest:latest -f cmd/app-manifest/Dockerfile .

docker/build/chat-roulette:
	docker build --platform linux/amd64 -t ghcr.io/chat-roulettte/chat-roulette:latest -f cmd/chat-roulette/Dockerfile .

docker/push/app-manifest:
	docker push ghcr.io/chat-roulettte/app-manifest:latest

db/run:
	docker run -d -p 5432:5432 --name postgres -e POSTGRES_DB=$(DATABASE) -e POSTGRES_PASSWORD=letmein docker.io/library/postgres:14.5

db/stop:
	docker rm -f postgres

migrate/create:
	@read -p "Enter migration filename: " FILENAME; \
	migrate create -ext sql -dir $(MIGRATIONS_DIR) $$FILENAME

migrate/up:
	migrate -database 'postgres://postgres:letmein@localhost:5432/$(DATABASE)?sslmode=disable' -path $(MIGRATIONS_DIR) up

migrate/down:
	migrate -database 'postgres://postgres:letmein@localhost:5432/$(DATABASE)?sslmode=disable' -path $(MIGRATIONS_DIR) down

migrate/force:
	@read -p "Enter force version: " VERSION; \
	migrate -database 'postgres://postgres:letmein@localhost:5432/$(DATABASE)?sslmode=disable' -path $(MIGRATIONS_DIR) force $$VERSION

ngrok:
	ngrok http 8080

dev/up:
	./scripts/dev.sh up

dev/destroy:
	./scripts/dev.sh destroy
