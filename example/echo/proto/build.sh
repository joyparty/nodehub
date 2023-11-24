#!/bin/sh
protoc --proto_path=. --proto_path=../../../proto \
	--go_out=. --go_opt=paths=source_relative \
	--go-grpc_out=. --go-grpc_opt=paths=source_relative \
	./server/services.proto ./server/echo/echo.proto
