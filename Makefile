# All application and extraction tools are implemented in Go.
export GOCACHE := $(CURDIR)/.cache/go-build

.PHONY: assets run build test vet android android-run reverse clean

assets:
	go run ./cmd/extract

run:
	go run .

build:
	go build -o bin/battlesquadron .

test:
	go test ./...

vet:
	go vet ./...

android:
	go run ./cmd/android

android-run:
	go run ./cmd/android -skip-bind -run

reverse:
	go run ./cmd/reverse

clean:
	go clean
