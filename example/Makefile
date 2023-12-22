SHELL:=/bin/sh

# paths
ROOT_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

# rules
.PHONY: build_protobuf
build_protobuf:
	(cd $(ROOT_DIR) && protoc \
		--proto_path=./chat/proto \
		--go_out=./chat/proto \
		--go_opt=paths=source_relative \
		--go-grpc_out=./chat/proto \
		--go-grpc_opt=paths=source_relative \
		./chat/proto/**/*.proto)

	(cd $(ROOT_DIR) && protoc \
		--proto_path=./echo/proto \
		--proto_path=./../api/protobuf \
		--go_out=./echo/proto \
		--go_opt=paths=source_relative \
		--go-grpc_out=./echo/proto \
		--go-grpc_opt=paths=source_relative \
		./echo/proto/**/*.proto)
