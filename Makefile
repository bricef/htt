
bin/htt: $(wildcard internal/**/*) cmd/htt/main.go go.mod go.sum
	go build -o bin/htt cmd/htt/main.go

clean:
	rm -rf bin

install:
	go install github.com/bricef/htt/cmd/htt

check:
	golangci-lint run

.PHONY: clean install check
