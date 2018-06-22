
bin/htt: main.go cmd/* models/* vars/*
	go build -o bin/htt

clean:
	rm -rf bin

.PHONY: clean
