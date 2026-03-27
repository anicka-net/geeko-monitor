BINARY  := geeko-monitor
VERSION := 0.2.0
PREFIX  := /usr

.PHONY: build test install clean

build:
	go build -o $(BINARY) .

test:
	go test -v ./...

install: build
	install -D -m 0755 $(BINARY) $(DESTDIR)$(PREFIX)/bin/$(BINARY)
	install -D -m 0644 $(BINARY).service $(DESTDIR)/usr/lib/systemd/system/$(BINARY).service

clean:
	rm -f $(BINARY)
