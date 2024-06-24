
build:
	go build -o dist/tfclean ./cmd/tfclean

install:
	go install github.com/takaishi/tfclean/cmd/tfclean
