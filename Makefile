.PHONY: all clean proto build test

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Protoc parameters
PROTOC=protoc
PROTO_DIR=proto
PROTO_GO_OUT=proto
PROTO_FILES=$(wildcard $(PROTO_DIR)/*.proto)
PROTO_GO_FILES=$(PROTO_FILES:.proto=.pb.go)

# Main binary name
BINARY_NAME=crdt-redis-server

all: deps proto build test

deps:
	$(GOMOD) download
	# Install protoc-gen-go if not already installed
	which protoc-gen-go || go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

proto: $(PROTO_GO_FILES)

%.pb.go: %.proto
	$(PROTOC) --proto_path=$(PROTO_DIR) \
		--go_out=$(PROTO_GO_OUT) --go_opt=paths=source_relative \
		$<

build:
	$(GOBUILD) -o $(BINARY_NAME) -v

test:
	$(GOTEST) -v ./...

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(PROTO_GO_FILES)

# Run the server
run: build
	./$(BINARY_NAME)

# Generate protobuf files only
proto-gen: clean-proto proto

# Clean protobuf generated files only
clean-proto:
	rm -f $(PROTO_GO_FILES)
