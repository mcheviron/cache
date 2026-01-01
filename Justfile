set positional-arguments

help:
    just -l

install:
    go mod download

fmt:
    go fmt ./...

check *args:
    go vet ./... "$@"

test *args:
    go test ./... "$@"

test-one name *args:
    go test ./... -run "$name" "$@"
