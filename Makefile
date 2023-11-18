SHELL:=/bin/sh

# paths
ROOT_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

# rules
.PHONY: build_protobuf
build_protobuf:
	protoc --proto_path=$(ROOT_DIR)proto --go_out=$(ROOT_DIR)proto --go_opt=paths=source_relative $(ROOT_DIR)proto/**/*.proto
