DIST_NAME=tiny-ssl-reverse-proxy
VERSION?=$(shell git describe --tags --always --dirty)

all: build

build:
	docker build -t tiny-ssl-reverse-proxy .
	docker run --rm tiny-ssl-reverse-proxy cat /go/bin/tiny-ssl-reverse-proxy > tiny-ssl-reverse-proxy
	chmod u+x tiny-ssl-reverse-proxy

install:
	go install

dist: dist/$(DIST_NAME)_darwin_amd64 dist/$(DIST_NAME)_linux_amd64

dist/$(DIST_NAME)_darwin_amd64:
	GOOS=darwin GOARCH=amd64 go build -o $@

dist/$(DIST_NAME)_linux_amd64:
	GOOS=linux GOARCH=amd64 go build -o $@

rel: dist
	hub release create -a dist $(VERSION)

fakecert: .FORCE
	openssl req \
		-x509 \
		-nodes \
		-sha256 \
		-newkey rsa:2048 \
		-keyout key.pem \
		-out crt.pem \
		-subj '/L=Earth/O=Fake Certificate/CN=localhost/' \
		-days 365

release:
	hub release create -a tiny-ssl-reverse-proxy_linux_amd64 -a tiny-ssl-reverse-proxy_darwin_amd64 $(shell git describe --tags --exact-match)


.FORCE:

.PHONY: all build install rel
