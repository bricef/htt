
bin/htt: $(wildcard internal/**/*) cmd/htt/main.go
	go build -o bin/htt cmd/htt/main.go

clean:
	rm -rf bin

install:
	go install github.com/bricef/htt/cmd/htt

.PHONY: clean install
