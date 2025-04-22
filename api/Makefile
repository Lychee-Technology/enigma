# Makefile for Go API

BINARY_NAME := enigma
PKG := ./cmd
DOCKER_IMAGE := lychee.technology/enigma:latest

.PHONY: all fmt lint test build run clean docker-build

all: fmt lint test build

fmt:
	go fmt ./...

lint:
	go vet ./...

test:
	go test ./... -cover

build:
	mkdir -p bin
	go build -o bin/$(BINARY_NAME) $(PKG)

run:
	./bin/$(BINARY_NAME)

clean:
	rm -rf bin coverage.out

