SHELL:=/bin/sh

# paths
ROOT_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

# rules
.PHONY: build_protobuf
build_protobuf:
	(cd $(ROOT_DIR) && protoc \
		--proto_path=./chat/api/protobuf \
		--go_out=./chat/proto \
		--go_opt=module=gitlab.haochang.tv/gopkg/nodehub/example/chat/proto \
		--go-grpc_out=./chat/proto \
		--go-grpc_opt=module=gitlab.haochang.tv/gopkg/nodehub/example/chat/proto \
		./chat/api/protobuf/**/*.proto)

	(cd $(ROOT_DIR) && protoc \
		--proto_path=./echo/api/protobuf \
		--proto_path=./../api/protobuf \
		--go_out=./echo/proto \
		--go_opt=module=gitlab.haochang.tv/gopkg/nodehub/example/echo/proto \
		--go-grpc_out=./echo/proto \
		--go-grpc_opt=module=gitlab.haochang.tv/gopkg/nodehub/example/echo/proto \
		./echo/api/protobuf/**/*.proto)
