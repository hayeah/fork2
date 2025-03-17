# Fork2 - Anthropic Claude Chat CLI

A command-line interface for interacting with Anthropic's Claude AI, built using the goo convention.

## Features

- Create and manage multiple chat sessions
- Save user messages and AI responses in a SQLite database
- Track token usage for each message
- Simple command-line interface

## Installation

1. Ensure you have Go installed on your system
2. Clone this repository
3. Run `make wire` to generate the dependency injection code
4. Run `make build` to build the application

## Configuration

Edit the `cfg.toml` file to configure the application:

```toml
[Database]
Dialect = "sqlite3"
DSN = "fork2.sqlite3"

[Logging]
LogLevel = "DEBUG"
LogFormat = "console"

[Anthropic]
APIKey = "your-anthropic-api-key-here"
```

Replace `your-anthropic-api-key-here` with your actual Anthropic API key.

## Usage

### Start a new chat

```
./fork2 new
```

You can also provide a title for the chat:

```
./fork2 new -t "My Chat Title"
```

### Send a message

```
./fork2 say "Hello, Claude! How are you today?"
```

### View the current chat

```
./fork2
```

### View a specific chat by ID

```
./fork2 chat 1
```

## Development

- Run `make wire` after making changes to the dependency injection setup
- Run `make run` to run the application with the current configuration
- Use `make dev` to run the application in development mode with automatic reloading
