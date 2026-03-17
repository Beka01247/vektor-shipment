PROTO_DIR := shared/proto
PROTO_SRC := $(wildcard $(PROTO_DIR)/*.proto)
GO_OUT := .

.PHONY: generate-proto
generate-proto:
	cd $(PROTO_DIR) && protoc \
		--go_out=. \
		--go_opt=paths=source_relative \
		--go-grpc_out=. \
		--go-grpc_opt=paths=source_relative \
		*.proto