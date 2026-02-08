default_path := "~/source"
default_depth := "3"

default:
    @just --list

build:
    go build -o pulse ./cmd/pulse

run *ARGS:
    go run ./cmd/pulse {{ARGS}}

scan path=default_path depth=default_depth:
    go run ./cmd/pulse --path {{path}} --depth {{depth}}

detail path=default_path depth=default_depth:
    go run ./cmd/pulse --path {{path}} --depth {{depth}} --detail

json path=default_path depth=default_depth:
    go run ./cmd/pulse --path {{path}} --depth {{depth}} --format json

time path=default_path depth=default_depth:
    go run ./cmd/pulse --path {{path}} --depth {{depth}} --time

full path=default_path depth=default_depth:
    go run ./cmd/pulse --path {{path}} --depth {{depth}} --detail --time

test:
    go test ./...

bench:
    go test -bench=. -benchmem ./...

vet:
    go vet ./...

lint: vet
    @echo "vet passed"

tidy:
    go mod tidy

clean:
    rm -f pulse

check: lint test
    @echo "all checks passed"
