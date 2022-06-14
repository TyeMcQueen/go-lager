all: test_go

test_go: testping.proto
	PATH="${HOME}/go/bin:${PATH}" protoc \
	  -I. \
		-I${HOME}/go/src \
		--go-grpc_out=. \
		--go_out=. \
		testping.proto
