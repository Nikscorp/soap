VERSION=$(shell git rev-parse --abbrev-ref HEAD)-$(shell git log -1 --format=%h)

build:
	@mkdir -p bin
	go build -ldflags "-X main.version=$(VERSION)" -o bin/ ./cmd/...

lint:
	golangci-lint run ./...

test:
	go test -count=1 -v ./...

test-cov:
	go test -coverprofile bin/cover.out -count=1 -v ./...
	go tool cover -html=bin/cover.out

test-race:
	go test -race -count=1 -v ./...

bench:
	@mkdir -p bin
	go test -bench=. -benchmem -count=10 -run=^$$ ./internal/pkg/imdbratings/... | tee bin/bench.txt

bench-real:
	@mkdir -p bin
	go test -tags imdbbench -bench=. -benchmem -count=10 -run=^$$ ./internal/pkg/imdbratings/... | tee bin/bench-real.txt

bench-baseline: bench
	@cp bin/bench.txt bin/bench-baseline.txt

bench-real-baseline: bench-real
	@cp bin/bench-real.txt bin/bench-real-baseline.txt

bench-stat:
	benchstat bin/bench-baseline.txt bin/bench.txt

bench-real-stat:
	benchstat bin/bench-real-baseline.txt bin/bench-real.txt

bench-tvmeta:
	@mkdir -p bin
	go test -bench=. -benchmem -count=10 -run=^$$ ./internal/pkg/tvmeta/... | tee bin/bench-tvmeta.txt

bench-tvmeta-baseline: bench-tvmeta
	@cp bin/bench-tvmeta.txt bin/bench-tvmeta-baseline.txt

bench-tvmeta-stat:
	benchstat bin/bench-tvmeta-baseline.txt bin/bench-tvmeta.txt

up:
	./bin/lazysoap

docker-build:
	docker compose build

docker-up:
	docker compose up

generate-mocks:
	go generate ./...

tidy:
	go mod tidy
	go mod vendor

clean:
	@rm -f bin/*
