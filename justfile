default: run

run:
    go run ./cmd/bridgery

gen:
    go generate ./...

test:
    go test ./...

lint:
    golangci-lint run ./...

fix: fmt
    go fix ./...
    golangci-lint run --fix ./...

fmt:
    treefmt

update:
    go get -u ./...
    go mod tidy
