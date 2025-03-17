.PHONY: wire run dev build

wire:
	go run github.com/google/wire/cmd/wire .

build:
	go build -o fork2 ./cmd

# make run ARGS="say 'Hello, how are you?'"
run:
	CONFIG_FILE=cfg.toml go run ./cmd $(ARGS)

dev:
	CONFIG_FILE=cfg.toml go run github.com/cortesi/modd/cmd/modd@latest
