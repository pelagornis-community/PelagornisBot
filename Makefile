.PHONY: build run tidy clean

build:
	go build -o bin/pelagornis ./cmd/pelagornis

run: build
	./bin/pelagornis

tidy:
	go mod tidy

clean:
	rm -f bin/pelagornis
