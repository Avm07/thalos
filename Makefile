
GO=go
PROGRAM=thalos-server
PREFIX=/usr/local
BINDIR=$(PREFIX)/bin
CFGDIR=$(PREFIX)/etc

.PHONY: build test

build: build/$(PROGRAM)

build/$(PROGRAM) :
	$(GO) build -o $@ cmd/thalos/main.go

install: build
	install -D build/$(PROGRAM) $(DESTDIR)$(BINDIR)/$(PROGRAM)
	install -m 644 -D config.example.yml $(DESTDIR)$(CFGDIR)/thalos/config.yml

test:
	$(GO) test -v ./...

clean :
	$(RM) -fr build
