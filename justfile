default:
    @just --list

build:
    go build -o pulse ./cmd/pulse

run *ARGS:
    go run ./cmd/pulse {{ARGS}}

test:
    go test ./...

bench:
    go test -bench=. -benchmem ./...

time:
    go run ./cmd/pulse --path /Users/guidefari/source/oss --depth 2 --time

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
