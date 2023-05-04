build:
	@go build -o bin/gopoker

run: build
	@./bin/gopoker

test:
	go test -v ./...