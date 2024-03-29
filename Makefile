SHELL:=/bin/sh

# paths
ROOT_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

# rules
.PHONY: build_protobuf
build_protobuf:
	(cd $(ROOT_DIR) && protoc \
		--proto_path=./api/protobuf \
		--go_out=./proto \
		--go_opt=module=github.com/joyparty/nodehub/proto \
		--go-grpc_out=./proto \
		--go-grpc_opt=module=github.com/joyparty/nodehub/proto \
		./api/protobuf/nodehub/*.proto)
