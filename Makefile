
bin/htt: htt/main.go cmd/* todo/* vars/* repo/* utils/*
	go build -o bin/htt htt/main.go

clean:
	rm -rf bin

.PHONY: clean
