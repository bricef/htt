
bin/ht: main.go cmd/*
	go build -o bin/ht

clean:
	rm -rf bin

.PHONY: clean