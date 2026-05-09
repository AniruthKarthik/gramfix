.PHONY: build install uninstall clean test

build:
	go build -o tmp/gramfix cmd/gramfix/main.go

install: build
	install -m 755 tmp/gramfix /usr/local/bin/gramfix

uninstall:
	rm -f /usr/local/bin/gramfix

clean:
	rm -rf tmp/

test:
	go test ./...
