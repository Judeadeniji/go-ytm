.PHONY: build run test air clean

AIR_BIN := $(shell go env GOPATH)/bin/air
APP_NAME := ytm-tui
ENTRY := cmd/ytm-tui/main.go

build:
	go build -o $(APP_NAME) $(ENTRY)

run: build
	./$(APP_NAME)

test:
	go test -v ./...

air:
	@if [ ! -f $(AIR_BIN) ]; then \
		echo "Installing air for live reloading..."; \
		go install github.com/air-verse/air@latest; \
	fi
	$(AIR_BIN) -c .air.toml

clean:
	rm -f $(APP_NAME)
	rm -rf tmp/
