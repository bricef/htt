
bin/htt: $(wildcard internal/**/*)
	go build -o bin/htt cmd/htt.go

clean:
	rm -rf bin

install:
	go get github.com/bricef/htt/htt

.PHONY: clean install
