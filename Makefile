.PHONY: build
build: test
	go build -o dist/tfclean ./cmd/tfclean

.PHONY: install
install:
	go install github.com/takaishi/tfclean/cmd/tfclean

.PHONY: test
test:
	go test -race ./...
