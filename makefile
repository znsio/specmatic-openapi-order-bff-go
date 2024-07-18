BFF_SERVICE_BINARY := specmatic-order-bff-go
CMD_DIR := ./cmd

.PHONY: all build clean

all: build

build:
	@echo "Building BFF Service"
	@go build -o $(BFF_SERVICE_BINARY) $(CMD_DIR)/main.go

clean:
	@echo "Cleaning up..."
	@rm $(BFF_SERVICE_BINARY)