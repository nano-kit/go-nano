.PHONY: test proto

test:
	gofmt -l -w -s .
	golint ./... | grep -v 'should have comment or be unexported' || true
	go test ./...

proto:
	cd ./cluster/clusterpb/proto/ && protoc --go_out=plugins=grpc:../ *.proto
