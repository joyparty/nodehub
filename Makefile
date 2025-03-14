SHELL:=/bin/sh

# paths
ROOT_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

# rules
.PHONY: build_protobuf
build_protobuf:
	(cd $(ROOT_DIR) && protoc \
		--proto_path=./api/protobuf \
		--go_out=./proto \
		--go_opt=default_api_level=API_OPAQUE \
		--go_opt=module=github.com/joyparty/nodehub/proto \
		--go-grpc_out=./proto \
		--go-grpc_opt=module=github.com/joyparty/nodehub/proto \
		./api/protobuf/nodehub/*.proto)

.PHONY: test
test:
	@go test -count=10 ./... | grep -v '^?'
