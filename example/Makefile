SHELL:=/bin/sh

# paths
ROOT_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

# rules
.PHONY: build_protobuf
build_protobuf:
	protoc \
		--proto_path=$(ROOT_DIR)chat/proto \
		--go_out=$(ROOT_DIR)chat/proto \
		--go_opt=paths=source_relative \
		--go-grpc_out=$(ROOT_DIR)chat/proto \
		--go-grpc_opt=paths=source_relative \
		$(ROOT_DIR)chat/proto/**/*.proto

	protoc \
		--proto_path=$(ROOT_DIR)echo/proto \
		--proto_path=$(ROOT_DIR)../proto \
		--go_out=$(ROOT_DIR)echo/proto \
		--go_opt=paths=source_relative \
		--go-grpc_out=$(ROOT_DIR)echo/proto \
		--go-grpc_opt=paths=source_relative \
		$(ROOT_DIR)echo/proto/**/*.proto
