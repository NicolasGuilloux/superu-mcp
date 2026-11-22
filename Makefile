install:
	go build -o $(HOME)/.local/bin/coursesu ./cmd/coursesu
	go build -o $(HOME)/.local/bin/superu-mcp ./cmd/superu-mcp

.PHONY: install
