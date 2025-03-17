package fork2

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/wire"
	"github.com/hayeah/goo"
	"github.com/hayeah/goo/fetch"
	"github.com/jmoiron/sqlx"

	_ "github.com/mattn/go-sqlite3" // Import SQLite driver
)

func ProvideConfig() (*Config, error) {
	cfg, err := goo.ParseConfig[Config]("")
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func ProvideGooConfig(cfg *Config) (*goo.Config, error) {
	return &cfg.Config, nil
}

// ProvideArgs parses cli args
func ProvideArgs() (*Args, error) {
	return goo.ParseArgs[Args]()
}

// ProvideChatStore creates a new ChatStore instance
func ProvideChatStore(db *sqlx.DB, logger *slog.Logger) *ChatStore {
	return &ChatStore{
		DB:     db,
		Logger: logger,
	}
}

// collect all the necessary providers
var Wires = wire.NewSet(
	goo.Wires,
	// provide the base config for goo library
	ProvideGooConfig,

	// app specific providers
	ProvideConfig,
	ProvideArgs,
	ProvideChatStore,

	// provide a goo.Runner interface for Main function, by using interface binding
	wire.Struct(new(App), "*"),
	wire.Bind(new(goo.Runner), new(*App)),
)

type Config struct {
	goo.Config
	Anthropic AnthropicConfig
}

type AnthropicConfig struct {
	APIKey string
}

type SayCmd struct {
	Message string `arg:"positional" help:"Message to send to the AI"`
}

type NewCmd struct {
	Title string `arg:"-t,--title" help:"Optional title for the new chat"`
}

type Args struct {
	Say  *SayCmd  `arg:"subcommand:say" help:"Add a new user message to the current chat"`
	New  *NewCmd  `arg:"subcommand:new" help:"Start a new chat"`
	Chat *ChatCmd `arg:"subcommand:chat" help:"View a specific chat by ID"`
}

type ChatCmd struct {
	ID int64 `arg:"positional" help:"Chat ID to view"`
}

type App struct {
	Args      *Args
	Config    *Config
	Shutdown  *goo.ShutdownContext
	DB        *sqlx.DB
	Migrator  *goo.DBMigrator
	Logger    *slog.Logger
	ChatStore *ChatStore
}

type ChatStore struct {
	DB     *sqlx.DB
	Logger *slog.Logger
}

type Chat struct {
	ID        int64     `db:"id"`
	Title     string    `db:"title"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

type Message struct {
	ID        int64     `db:"id"`
	ChatID    int64     `db:"chat_id"`
	Role      string    `db:"role"`
	Content   string    `db:"content"`
	Tokens    int       `db:"tokens"`
	CreatedAt time.Time `db:"created_at"`
}

func (cs *ChatStore) CreateChat(title string) (int64, error) {
	if title == "" {
		title = fmt.Sprintf("Chat %s", time.Now().Format("2006-01-02 15:04:05"))
	}

	now := time.Now()
	result, err := cs.DB.Exec(
		"INSERT INTO chats (title, created_at, updated_at) VALUES (?, ?, ?)",
		title, now, now,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to create chat: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	return id, nil
}

func (cs *ChatStore) GetCurrentChatID() (int64, error) {
	var id int64
	err := cs.DB.Get(&id, "SELECT id FROM chats ORDER BY id DESC LIMIT 1")
	if err != nil {
		if err == sql.ErrNoRows {
			// Create a new chat if none exists
			return cs.CreateChat("")
		}
		return 0, fmt.Errorf("failed to get current chat ID: %w", err)
	}
	return id, nil
}

func (cs *ChatStore) GetChat(id int64) (*Chat, error) {
	var chat Chat
	err := cs.DB.Get(&chat, "SELECT * FROM chats WHERE id = ?", id)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat: %w", err)
	}
	return &chat, nil
}

func (cs *ChatStore) GetMessages(chatID int64) ([]Message, error) {
	var messages []Message
	err := cs.DB.Select(&messages, "SELECT * FROM messages WHERE chat_id = ? ORDER BY id", chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}
	return messages, nil
}

func (cs *ChatStore) AddMessage(chatID int64, role, content string, tokens int) error {
	_, err := cs.DB.Exec(
		"INSERT INTO messages (chat_id, role, content, tokens, created_at) VALUES (?, ?, ?, ?, ?)",
		chatID, role, content, tokens, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("failed to add message: %w", err)
	}

	// Update chat's updated_at timestamp
	_, err = cs.DB.Exec("UPDATE chats SET updated_at = ? WHERE id = ?", time.Now(), chatID)
	if err != nil {
		return fmt.Errorf("failed to update chat timestamp: %w", err)
	}

	return nil
}

func (cs *ChatStore) ListChats() ([]Chat, error) {
	var chats []Chat
	err := cs.DB.Select(&chats, "SELECT * FROM chats ORDER BY updated_at DESC")
	if err != nil {
		return nil, fmt.Errorf("failed to list chats: %w", err)
	}
	return chats, nil
}

func (app *App) Run() error {
	// Run migrations
	err := app.Migrator.Up([]goo.Migration{
		{
			Name: "create_chats_table",
			Up: `
				CREATE TABLE IF NOT EXISTS chats (
					id INTEGER PRIMARY KEY,
					title TEXT NOT NULL,
					created_at TIMESTAMP NOT NULL,
					updated_at TIMESTAMP NOT NULL
				);
			`,
		},
		{
			Name: "create_messages_table",
			Up: `
				CREATE TABLE IF NOT EXISTS messages (
					id INTEGER PRIMARY KEY,
					chat_id INTEGER NOT NULL,
					role TEXT NOT NULL,
					content TEXT NOT NULL,
					tokens INTEGER NOT NULL,
					created_at TIMESTAMP NOT NULL,
					FOREIGN KEY (chat_id) REFERENCES chats (id)
				);
			`,
		},
	})

	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	args := app.Args

	switch {
	case args.Say != nil:
		return app.handleSayCommand(args.Say.Message)
	case args.New != nil:
		return app.handleNewCommand(args.New.Title)
	case args.Chat != nil:
		return app.handleChatCommand(args.Chat.ID)
	default:
		// Default command: dump the current chat
		return app.handleDefaultCommand()
	}
}

func (app *App) handleSayCommand(message string) error {
	chatID, err := app.ChatStore.GetCurrentChatID()
	if err != nil {
		return fmt.Errorf("failed to get current chat: %w", err)
	}

	// Add user message to the database (estimate tokens for now)
	userTokens := estimateTokens(message)
	err = app.ChatStore.AddMessage(chatID, "user", message, userTokens)
	if err != nil {
		return fmt.Errorf("failed to add user message: %w", err)
	}

	// Call Anthropic API
	response, tokens, err := app.callAnthropicAPI(chatID)
	if err != nil {
		return fmt.Errorf("failed to call Anthropic API: %w", err)
	}

	// Add assistant message to the database
	err = app.ChatStore.AddMessage(chatID, "assistant", response, tokens)
	if err != nil {
		return fmt.Errorf("failed to add assistant message: %w", err)
	}

	// Print the response
	fmt.Println(response)

	return nil
}

func (app *App) handleNewCommand(title string) error {
	chatID, err := app.ChatStore.CreateChat(title)
	if err != nil {
		return fmt.Errorf("failed to create new chat: %w", err)
	}

	fmt.Printf("Created new chat with ID: %d\n", chatID)
	return nil
}

func (app *App) handleChatCommand(chatID int64) error {
	chat, err := app.ChatStore.GetChat(chatID)
	if err != nil {
		return fmt.Errorf("failed to get chat: %w", err)
	}

	messages, err := app.ChatStore.GetMessages(chatID)
	if err != nil {
		return fmt.Errorf("failed to get messages: %w", err)
	}

	fmt.Printf("Chat: %s (ID: %d)\n", chat.Title, chat.ID)
	fmt.Println("Created at:", chat.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Println("Updated at:", chat.UpdatedAt.Format("2006-01-02 15:04:05"))
	fmt.Println("Messages:")
	fmt.Println("=========")

	for _, msg := range messages {
		fmt.Printf("[%s] (%d tokens)\n", msg.Role, msg.Tokens)
		fmt.Println(msg.Content)
		fmt.Println("=========")
	}

	return nil
}

func (app *App) handleDefaultCommand() error {
	chatID, err := app.ChatStore.GetCurrentChatID()
	if err != nil {
		return fmt.Errorf("failed to get current chat: %w", err)
	}

	return app.handleChatCommand(chatID)
}

func (app *App) callAnthropicAPI(chatID int64) (string, int, error) {
	// Get previous messages for context
	messages, err := app.ChatStore.GetMessages(chatID)
	if err != nil {
		return "", 0, fmt.Errorf("failed to get previous messages: %w", err)
	}

	// Format messages for API
	apiMessages := make([]map[string]string, 0, len(messages))
	for _, msg := range messages {
		apiMessages = append(apiMessages, map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}

	// Call Anthropic API
	opts := &fetch.Options{
		BaseURL: "https://api.anthropic.com",
		Header: http.Header{
			"Content-Type":      {"application/json"},
			"x-api-key":         {app.Config.Anthropic.APIKey},
			"anthropic-version": {"2023-06-01"},
		},
		Logger: app.Logger,
	}

	resp, err := opts.JSON("POST", "/v1/messages", &fetch.Options{
		Body: `{
			"model": "claude-3-opus-20240229",
			"messages": {{Messages}},
			"max_tokens": 4096
		}`,
		BodyParams: map[string]any{
			"Messages": apiMessages,
		},
	})
	if err != nil {
		return "", 0, fmt.Errorf("API request failed: %w", err)
	}

	// Extract the response content and token usage
	content := resp.Get("content.0.text").String()
	inputTokens := int(resp.Get("usage.input_tokens").Int())
	outputTokens := int(resp.Get("usage.output_tokens").Int())
	totalTokens := inputTokens + outputTokens

	return content, totalTokens, nil
}

// Simple function to estimate tokens (very rough estimate)
func estimateTokens(text string) int {
	// Rough estimate: 1 token ≈ 4 characters
	return len(text) / 4
}

// Helper function to convert API messages to the format expected by the API
func formatMessagesForAPI(messages []Message) []map[string]string {
	apiMessages := make([]map[string]string, 0, len(messages))
	for _, msg := range messages {
		apiMessages = append(apiMessages, map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}
	return apiMessages
}
