package fork2

import (
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"bytes"

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

	prompt *Prompt
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
	ID           int64     `db:"id"`
	ChatID       int64     `db:"chat_id"`
	Role         string    `db:"role"`
	Content      string    `db:"content"`
	InputTokens  int       `db:"input_tokens"`
	OutputTokens int       `db:"output_tokens"`
	TotalTokens  int       `db:"total_tokens"`
	CacheHit     bool      `db:"cache_hit"`
	CreatedAt    time.Time `db:"created_at"`
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

func (cs *ChatStore) AddMessage(chatID int64, role, content string, inputTokens, outputTokens int, cacheHit bool) error {
	totalTokens := inputTokens + outputTokens
	_, err := cs.DB.Exec(
		"INSERT INTO messages (chat_id, role, content, input_tokens, output_tokens, total_tokens, cache_hit, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		chatID, role, content, inputTokens, outputTokens, totalTokens, cacheHit, time.Now(),
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
					input_tokens INTEGER NOT NULL,
					output_tokens INTEGER NOT NULL,
					total_tokens INTEGER NOT NULL,
					cache_hit BOOLEAN NOT NULL DEFAULT 0,
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

	// Update prompt on startup
	err = app.updatePrompt()
	if err != nil {
		return fmt.Errorf("failed to load the prompt: %w", err)
	}

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

	// Call Anthropic API with streaming - this will print the response as it comes in
	response, inputTokens, outputTokens, cacheHit, err := app.callAnthropicAPI(chatID, message)
	if err != nil {
		return fmt.Errorf("failed to call Anthropic API: %w", err)
	}

	// Now that we have a successful response, save both the user message and the response
	err = app.ChatStore.AddMessage(chatID, "user", message, inputTokens, 0, false)
	if err != nil {
		return fmt.Errorf("failed to add user message: %w", err)
	}

	// Add assistant message to the database
	err = app.ChatStore.AddMessage(chatID, "assistant", response, 0, outputTokens, cacheHit)
	if err != nil {
		return fmt.Errorf("failed to add assistant message: %w", err)
	}

	// No need to print the response again as it was already printed during streaming
	// Just add a newline for better formatting
	fmt.Println()

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
		cacheStatus := ""
		if msg.Role == "assistant" && msg.CacheHit {
			cacheStatus = " (cached)"
		}

		if msg.Role == "user" {
			fmt.Printf("[%s] (%d input tokens)\n", msg.Role, msg.InputTokens)
		} else {
			fmt.Printf("[%s] (%d output tokens%s)\n", msg.Role, msg.OutputTokens, cacheStatus)
		}
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

func (app *App) callAnthropicAPI(chatID int64, userMessage string) (string, int, int, bool, error) {
	// Get previous messages for context
	messages, err := app.ChatStore.GetMessages(chatID)
	if err != nil {
		return "", 0, 0, false, fmt.Errorf("failed to get previous messages: %w", err)
	}

	// Format messages for API
	apiMessages := make([]map[string]string, 0, len(messages))
	for _, msg := range messages {
		apiMessages = append(apiMessages, map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}

	// Add the current user message for the API call
	apiMessages = append(apiMessages, map[string]string{
		"role":    "user",
		"content": userMessage,
	})

	// Call Anthropic API with streaming enabled
	opts := &fetch.Options{
		BaseURL: "https://api.anthropic.com",
		Header: http.Header{
			"Content-Type":      {"application/json"},
			"x-api-key":         {app.Config.Anthropic.APIKey},
			"anthropic-version": {"2023-06-01"},
		},
		Logger: app.Logger,
	}

	// Prepare the request body with stream=true
	sseResp, err := opts.SSE("POST", "/v1/messages", &fetch.Options{
		Body: `{
			"model": "claude-3-opus-20240229",
			"messages": {{Messages}},
			"max_tokens": 4096,
			"stream": true
		}`,
		BodyParams: map[string]any{
			"Messages": apiMessages,
		},
		Logger: app.Logger,
	})
	if err != nil {
		return "", 0, 0, false, fmt.Errorf("API request failed: %w", err)
	}

	// Open a file to tee the response. ./tmp/transcript-*.log
	transcriptFile, err := os.CreateTemp("./tmp", "transcript-*.log")
	if err != nil {
		return "", 0, 0, false, fmt.Errorf("failed to create transcript file: %w", err)
	}
	defer transcriptFile.Close()

	// Tee the response to a file for debugging purposes
	sseResp.Tee(transcriptFile)

	// Create an SSEReader to process the streaming response
	reader := NewAnthropicStreamReader(sseResp)
	defer reader.Close()

	// Create a buffer to store the content
	var contentBuffer bytes.Buffer

	// Use io.TeeReader to copy the stream to both the buffer and stdout
	teeReader := io.TeeReader(reader, &contentBuffer)

	// Copy from the teeReader to stdout
	_, err = io.Copy(os.Stdout, teeReader)
	if err != nil && err != io.EOF {
		return "", 0, 0, false, fmt.Errorf("error reading stream: %w", err)
	}

	// Get the final content
	content := contentBuffer.String()

	// Get token usage directly from the stream reader
	inputTokens, outputTokens := reader.TokenUsage()

	// Check if this was a cache hit (Anthropic doesn't provide this directly, so we'll assume false for now)
	cacheHit := false

	return content, inputTokens, outputTokens, cacheHit, nil
}

// updatePrompt sets the current prompt to the default prompt
func (app *App) updatePrompt() error {
	prompt, err := DefaultPrompt()
	if err != nil {
		return fmt.Errorf("failed to create default prompt: %w", err)
	}

	app.prompt = prompt
	return nil
}

// AnthropicStreamReader implements io.Reader interface for SSE events
type AnthropicStreamReader struct {
	sseResp      *fetch.SSEResponse
	buffer       bytes.Buffer
	err          error
	inputTokens  int
	outputTokens int
}

// NewAnthropicStreamReader creates a Reader to stream the text of a message
func NewAnthropicStreamReader(sseResp *fetch.SSEResponse) *AnthropicStreamReader {
	return &AnthropicStreamReader{
		sseResp:      sseResp,
		inputTokens:  0,
		outputTokens: 0,
	}
}

// Read implements the io.Reader interface
func (r *AnthropicStreamReader) Read(p []byte) (n int, err error) {
	// If we have data in the buffer, return it
	if r.buffer.Len() > 0 {
		return r.readFromBuffer(p)
	}

	// If we've encountered an error before, return it
	if r.err != nil {
		return 0, r.err
	}

	// Try to get the next event
	if !r.sseResp.Next() {
		// Check if there was an error during scanning
		if err := r.sseResp.Err(); err != nil {
			r.err = err
			return 0, err
		}
		// No more events, return EOF
		r.err = io.EOF
		return 0, io.EOF
	}

	// Get the event and process it
	event := r.sseResp.Event()

	// Process the event data based on event type
	switch event.Event {
	case "message_start":
		// Extract input tokens from message_start event
		inputTokens := event.GJSON("message.usage.input_tokens").Int()
		if inputTokens > 0 {
			r.inputTokens = int(inputTokens)
		}
	case "content_block_start":
		// Nothing to do here, just acknowledging the start of a content block
	case "content_block_delta":
		// Extract the text from the delta
		text := event.GJSON("delta.text").String()
		if text != "" {
			r.buffer.WriteString(text)
		}
	case "content_block_stop":
		// Nothing to do here, just acknowledging the end of a content block
	case "message_delta":
		// Update output tokens from message_delta event
		outputTokens := event.GJSON("usage.output_tokens").Int()
		if outputTokens > 0 {
			r.outputTokens = int(outputTokens)
		}
	case "message_stop":
		// End of message, we'll return what's in the buffer and then EOF on next call
		if r.buffer.Len() == 0 {
			r.err = io.EOF
			return 0, io.EOF
		}
	case "error":
		// Handle error events
		errorType := event.GJSON("error.type").String()
		errorMsg := event.GJSON("error.message").String()

		// Set the error and return it
		r.err = fmt.Errorf("anthropic stream error: %s - %s", errorType, errorMsg)
		return 0, r.err
	}

	// If we have data in the buffer now, return it
	if r.buffer.Len() > 0 {
		return r.readFromBuffer(p)
	}

	// No data in this event, try again
	return r.Read(p)
}

// readFromBuffer reads data from the internal buffer into p
func (r *AnthropicStreamReader) readFromBuffer(p []byte) (n int, err error) {
	// Read directly from the buffer into p
	// bytes.Buffer.Read already handles the case where len(p) < buffer.Len()
	return r.buffer.Read(p)
}

// TokenUsage returns the input and output token counts
func (r *AnthropicStreamReader) TokenUsage() (int, int) {
	return r.inputTokens, r.outputTokens
}

// Close closes the underlying SSE response
func (r *AnthropicStreamReader) Close() error {
	return r.sseResp.Close()
}
