
bin/htt: main.go cmd/* todo/* vars/* repo/* utils/*
	go build -o bin/htt

clean:
	rm -rf bin

.PHONY: clean
