VERSION=$(shell git rev-parse --abbrev-ref HEAD)-$(shell git log -1 --format=%h)

build:
	@mkdir -p bin
	go build -pgo=auto -ldflags "-X github.com/Nikscorp/soap/internal/pkg/trace.Version=$(VERSION)" -o bin/ ./cmd/...

lint:
	golangci-lint run ./...

test:
	go test -count=1 -v ./...

test-cov:
	go test -coverprofile bin/cover.out -count=1 -v ./...
	go tool cover -html=bin/cover.out

test-race:
	go test -race -count=1 -v ./...

up:
	./bin/lazysoap --api-key=$(API_KEY)

docker-build:
	docker-compose build

docker-up:
	docker-compose up

generate-mocks:
	go generate ./...

tidy:
	go mod tidy
	go mod vendor

install-go-swagger:
	go install github.com/go-swagger/go-swagger/cmd/swagger@latest

generate-swagger:
	swagger generate spec -o ./swagger/swagger.yaml --scan-models

clean:
	@rm -f bin/*
