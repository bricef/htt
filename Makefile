
bin/htt: htt/main.go cmd/* todo/* vars/* repo/* utils/*
	go build -o bin/htt htt/main.go

clean:
	rm -rf bin

install:
	go get github.com/hypotheticalco/tracker-client/htt

.PHONY: clean install
