# All application and extraction tools are implemented in Go.
export GOCACHE := $(CURDIR)/.cache/go-build

.PHONY: assets demo run build test vet benchmark android android-run reverse clean

assets:
	go run ./cmd/extract
	go run ./cmd/demo

demo:
	go run ./cmd/demo

run:
	go run .

build:
	go build -o bin/battlesquadron .

test:
	go test ./...

vet:
	go vet ./...

benchmark:
	go run ./cmd/benchmark

android:
	go run ./cmd/android

android-run:
	go run ./cmd/android -skip-bind -run

reverse:
	go run ./cmd/reverse

clean:
	go clean
