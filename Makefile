
bin/ht: main.go cmd/* models/* vars/*
	go build -o bin/ht

clean:
	rm -rf bin

.PHONY: clean
