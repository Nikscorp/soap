build:
	@mkdir -p bin
	go build -o bin/ ./cmd/...

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

generate:
	go generate ./...

tidy:
	go mod tidy
	go mod vendor

clean:
	@rm -f bin/*
