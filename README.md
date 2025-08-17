# Telegram Logger for Go

A high-performance, production-ready logging library that sends logs to Telegram groups with topic support. Perfect for
monitoring applications, debugging, and receiving real-time notifications.

## Features

- 🚀 **High Performance** - Optimized with connection pooling, buffering, and async operations
- 📱 **Telegram Integration** - Send logs directly to Telegram groups and topics
- 🎯 **Multi-Level Logging** - Support for Info, Warn, Error, and Debug levels
- 🔄 **Auto-Retry Logic** - Resilient message delivery with exponential backoff
- ⚡ **Async Flushing** - Optional periodic flushing for better performance
- 🎨 **Color-Coded Terminal** - Beautiful terminal output with color coding
- 🐳 **Docker Support** - Automatic environment detection
- 🛡️ **Graceful Shutdown** - Ensures all logs are sent before termination
- 📊 **Batching** - Intelligent message chunking for Telegram's limits

## Installation

```bash
go get github.com/your-username/logtg
```

## Quick Start

### 1. Environment Setup

Create a `.env` file in your project root:

```env
TELEGRAM_BOT_TOKEN=your_bot_token_here
TELEGRAM_GROUP_ID=-1001234567890
TELEGRAM_GROUP_INFO_TOPIC_ID=123
TELEGRAM_GROUP_ERROR_TOPIC_ID=456
TELEGRAM_GROUP_WARN_TOPIC_ID=789
TELEGRAM_GROUP_DEBUG_TOPIC_ID=101
```

### 2. Basic Usage

```go
package main

import "github.com/your-username/logtg"

func main() {
	// Initialize logger
	logger := logtg.InitLog()
	defer logger.Close() // Ensure graceful shutdown

	// Log messages
	logger.Info("Application started successfully")
	logger.Warn("This is a warning message")
	logger.Error("Something went wrong: %s", "error details")
	logger.Debug("Debug information: %v", someVariable)

	// Log with error object
	err := someFunction()
	if err != nil {
		logger.ErrorWithError(err, "Failed to execute function")
	}

	// Manual flush (optional - auto-flush is enabled)
	logger.Flush()
}
```

## Configuration

### Environment Variables

| Variable                        | Required | Description                   |
|---------------------------------|----------|-------------------------------|
| `TELEGRAM_BOT_TOKEN`            | ✅        | Your Telegram bot token       |
| `TELEGRAM_GROUP_ID`             | ✅        | Target group chat ID          |
| `TELEGRAM_GROUP_INFO_TOPIC_ID`  | ✅        | Topic ID for info messages    |
| `TELEGRAM_GROUP_ERROR_TOPIC_ID` | ✅        | Topic ID for error messages   |
| `TELEGRAM_GROUP_WARN_TOPIC_ID`  | ❌        | Topic ID for warning messages |
| `TELEGRAM_GROUP_DEBUG_TOPIC_ID` | ❌        | Topic ID for debug messages   |

### Advanced Configuration

```go
config := logtg.TelegramConfig{
BotToken:     "your_bot_token",
DefaultGroup: -1001234567890,
GroupIDByType: map[string]int64{
"info":  -1001234567890,
"error": -1001234567891, // Different group for errors
},
TopicIDByType: map[string]int64{
"info":  123,
"error": 456,
},
EnableAsync:   true,
FlushInterval: 10 * time.Second,
HTTPTimeout:   15 * time.Second,
}

logger := logtg.NewLogger(config)
```

## API Reference

### Logging Methods

```go
// Standard logging methods
logger.Info(format string, v ...interface{})
logger.Warn(format string, v ...interface{})
logger.Error(format string, v ...interface{})
logger.Debug(format string, v ...interface{})

// Special methods
logger.OnlyInfo(format string, v ...interface{}) // Logs to terminal only
logger.ErrorWithError(err error, format string, v ...) // Logs with error context

// Control methods
logger.Flush()       // Manual flush to Telegram
logger.Close() error // Graceful shutdown
```

### Log Levels

- **Info** 🟢 - General information (green in terminal)
- **Warn** 🟡 - Warning messages (yellow in terminal)
- **Error** 🔴 - Error messages (red in terminal)
- **Debug** 🔵 - Debug information (cyan in terminal)

## Getting Started with Telegram Bot

### 1. Create a Telegram Bot

1. Message [@BotFather](https://t.me/botfather) on Telegram
2. Send `/newbot` and follow instructions
3. Save the bot token

### 2. Setup Group and Topics

1. Create a Telegram group
2. Add your bot to the group
3. Make the bot an admin
4. Enable topics in group settings
5. Create topics for different log levels
6. Get group ID and topic IDs using [@userinfobot](https://t.me/userinfobot)

### 3. Get Chat IDs

Forward a message from your group to [@userinfobot](https://t.me/userinfobot) to get:

- Group ID (negative number)
- Topic IDs (positive numbers)

## Performance Features

### Async Flushing

```go
// Enable automatic periodic flushing
config.EnableAsync = true
config.FlushInterval = 5 * time.Second
```

### Batching and Chunking

- Automatically batches messages for efficiency
- Respects Telegram's 4000 character limit
- Intelligent message splitting

### Connection Pooling

- Reuses HTTP connections
- Configurable timeouts
- Concurrent request limiting

### Retry Logic

- Exponential backoff
- Configurable retry attempts
- Error recovery

## Docker Support

The library automatically detects Docker environment and skips `.env` file loading:

```dockerfile
FROM golang:alpine
WORKDIR /app
COPY . .
ENV TELEGRAM_BOT_TOKEN=your_token
ENV TELEGRAM_GROUP_ID=-1001234567890
# ... other env vars
RUN go build -o app
CMD ["./app"]
```

## Examples

### Web Server Logging

```go
func main() {
logger := logtg.InitLog()
defer logger.Close()

http.HandleFunc("/", func (w http.ResponseWriter, r *http.Request) {
logger.Info("Request: %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
// Handle request...
})

logger.Info("Server starting on :8080")
if err := http.ListenAndServe(":8080", nil); err != nil {
logger.ErrorWithError(err, "Server failed to start")
}
}
```

### Database Operations

```go
func processUser(userID int) error {
logger := logtg.InitLog()

logger.Debug("Processing user: %d", userID)

user, err := db.GetUser(userID)
if err != nil {
logger.ErrorWithError(err, "Failed to get user %d", userID)
return err
}

logger.Info("Successfully processed user: %s", user.Name)
return nil
}
```

### Background Jobs

```go
func backgroundWorker() {
logger := logtg.InitLog()
defer logger.Close()

ticker := time.NewTicker(1 * time.Hour)
defer ticker.Stop()

for {
select {
case <-ticker.C:
logger.Info("Starting hourly cleanup job")
if err := cleanupOldData(); err != nil {
logger.ErrorWithError(err, "Cleanup job failed")
} else {
logger.Info("Cleanup job completed successfully")
}
}
}
}
```

## Best Practices

### 1. Use Structured Logging

```go
// Good
logger.Info("User login: userID=%d, email=%s, ip=%s", userID, email, ip)

// Avoid
logger.Info("User " + email + " logged in from " + ip)
```

### 2. Handle Errors Properly

```go
if err != nil {
logger.ErrorWithError(err, "Operation failed for user %d", userID)
return err
}
```

### 3. Use Appropriate Log Levels

```go
logger.Debug("Cache hit for key: %s", key) // Development info
logger.Info("User registered: %s", email) // Important events
logger.Warn("Rate limit approached: %d/100", count) // Potential issues
logger.Error("Database connection failed") // Serious problems
```

### 4. Graceful Shutdown

```go
func main() {
logger := logtg.InitLog()
defer logger.Close() // Always ensure graceful shutdown

// Your application code...
}
```

## Troubleshooting

### Common Issues

1. **Bot not receiving messages**
    - Ensure bot is added to the group
    - Verify bot has admin permissions
    - Check group ID is negative number

2. **Topic messages not working**
    - Ensure topics are enabled in group
    - Verify topic IDs are correct
    - Check bot has permission to post in topics

3. **Rate limiting**
    - Telegram has rate limits (30 messages/second)
    - Use async flushing to batch messages
    - Consider increasing flush interval

### Debug Mode

```go
// Enable debug logging to see internal operations
logger.Debug("Debug mode enabled")
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Support

- 📧 Email: logtg@akbarali.uz
- 🐛 Issues: [GitHub Issues](https://github.com/akbarali1/logtg/issues)
