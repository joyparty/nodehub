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

.PHONY: run_echo_gateway
run_echo_gateway:
	(cd $(ROOT_DIR)echo/gateway && go run .)

.PHONY: run_echo_rpc
run_echo_rpc:
	(cd $(ROOT_DIR)echo/rpc && go run .)

.PHONY: run_chat_gateway
run_chat_gateway:
	(cd $(ROOT_DIR)chat/gateway && go run .)

.PHONY: run_chat_rpc
run_chat_rpc:
	(cd $(ROOT_DIR)chat/room && go run .)
