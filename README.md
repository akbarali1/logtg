# Telegram Logger for Go

A powerful and flexible Telegram logging library for Go applications. Send all your logs directly to Telegram channels
with support for different log levels, topic-based routing, and `io.MultiWriter` compatibility.

## 🌟 Key Features

- 📱 **Telegram Integration** - Send logs directly to Telegram groups/channels
- 🎯 **Topic-based Routing** - Separate topics for different log levels
- 🔄 **Backward Compatible** - Add to existing projects without breaking changes
- 📝 **MultiWriter Support** - Full `log.SetOutput(io.MultiWriter(...))` compatibility
- ⚡ **Async/Batch Processing** - High performance with batch message sending
- 🎨 **Colorized Terminal Output** - Different colors for different log levels
- 🔒 **Thread-Safe** - Safe for concurrent use
- 🔄 **Auto-retry** - Automatic retry mechanism for Telegram API failures
- 🛡️ **Graceful Shutdown** - Proper cleanup on application exit
- 📊 **Buffering & Chunking** - Efficient message batching and splitting

## 📦 Installation

```bash
go get github.com/akbarali1/logtg
```

## ⚙️ Configuration

### Environment Variables

Create a `.env` file or set environment variables:

```env
TELEGRAM_BOT_TOKEN=your_bot_token_here
TELEGRAM_GROUP_ID=-1001234567890
TELEGRAM_GROUP_INFO_TOPIC_ID=123
TELEGRAM_GROUP_ERROR_TOPIC_ID=456
TELEGRAM_GROUP_WARN_TOPIC_ID=789
TELEGRAM_GROUP_DEBUG_TOPIC_ID=012
```

### Telegram Bot Setup

1. Contact [@BotFather](https://t.me/botfather)
2. Create a new bot with `/newbot`
3. Get the bot token
4. Add the bot to your group/channel and make it an admin
5. Get the group/channel ID

### Docker Support

The library automatically detects Docker environment and skips `.env` file loading when running in containers.

## 🚀 Usage

### 1. Simple Usage (Traditional API)

```go
package main

import (
	"errors"
	"your-project/logtg"
)

func main() {
	// Initialize logger
	logger := logtg.InitLog()
	defer logger.Close()

	// Different log levels
	logger.Info("Application started")
	logger.Warn("This is a warning message")
	logger.Error("This is an error message")
	logger.Debug("Debug information")

	// Console-only message (won't send to Telegram)
	logger.OnlyInfo("This appears only in terminal")

	// Error with additional error object
	err := errors.New("sample error")
	logger.ErrorWithError(err, "An error occurred: %s", "details")

	// Manual flush
	logger.Flush()
}
```

### 2. MultiWriter Integration

```go
package main

import (
	"io"
	"log"
	"os"
	"your-project/logtg"
)

func main() {
	logger := logtg.InitLog()
	defer logger.Close()

	// Setup MultiWriter (outputs to both stderr and Telegram)
	logger.SetupMultiWriter("info", false) // autoFlush = false

	// Use standard log package
	log.Println("This goes to console and Telegram")
	log.Printf("Formatted message: %d", 123)

	// Manual flush
	logger.Flush()
}
```

### 3. Custom Multiple Outputs

```go
// Write to file, console, and Telegram
logFile, _ := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
outputs := []io.Writer{os.Stdout, logFile}

logger.SetupMultiWriterWithCustomOutput(outputs, "error", true) // autoFlush = true

log.Println("This writes to 3 places: console, file, and Telegram")
```

### 4. Different Log Levels with Custom Loggers

```go
// Create separate loggers for different levels
infoLogger := log.New(
io.MultiWriter(os.Stdout, logger.GetLogBuffer("info", false)),
"[INFO] ",
log.LstdFlags,
)

errorLogger := log.New(
io.MultiWriter(os.Stderr, logger.GetLogBuffer("error", true)), // Auto flush
"[ERROR] ",
log.LstdFlags,
)

warnLogger := log.New(
io.MultiWriter(os.Stdout, logger.GetLogBuffer("warn", false)),
"[WARN] ",
log.LstdFlags,
)

infoLogger.Println("Info message")
errorLogger.Println("Error message") // Immediately sent to Telegram
warnLogger.Println("Warning message")

// Manual flush for non-auto-flush buffers
logger.Flush()
```

### 5. HTTP Middleware Example

```go
func LoggingMiddleware(tg *logtg.Logger) func (http.Handler) http.Handler {
logBuffer := tg.GetLogBuffer("info", true) // Auto flush
logger := log.New(io.MultiWriter(os.Stdout, logBuffer), "[HTTP] ", log.LstdFlags)

return func (next http.Handler) http.Handler {
return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
start := time.Now()
next.ServeHTTP(w, r)
duration := time.Since(start)

logger.Printf("%s %s %v", r.Method, r.RequestURI, duration)
})
}
}

// Usage in HTTP server
func main() {
logger := logtg.InitLog()
defer logger.Close()

mux := http.NewServeMux()
mux.HandleFunc("/", homeHandler)

// Apply logging middleware
handler := LoggingMiddleware(logger)(mux)

http.ListenAndServe(":8080", handler)
}
```

### 6. Structured Logging

```go
import "encoding/json"

func setupStructuredLogging(tg *logtg.Logger) {
jsonBuffer := tg.GetLogBuffer("info", false)

type LogEntry struct {
Timestamp string      `json:"timestamp"`
Level     string      `json:"level"`
Message   string      `json:"message"`
Data      interface{} `json:"data,omitempty"`
}

logEntry := LogEntry{
Timestamp: time.Now().Format(time.RFC3339),
Level:     "INFO",
Message:   "User login successful",
Data: map[string]interface{}{
"user_id": 123,
"ip":      "192.168.1.1",
},
}

jsonData, _ := json.Marshal(logEntry)
jsonBuffer.Write(jsonData)
jsonBuffer.Write([]byte("\n"))

jsonBuffer.Flush()
tg.Flush()
}
```

### 7. Asynchronous Logging

```go
func setupAsyncLogging(tg *logtg.Logger) {
logBuffer := tg.GetLogBuffer("info", false)

// Channel for async logging
logChan := make(chan string, 100)

// Start goroutine
go func () {
for msg := range logChan {
logBuffer.Write([]byte(msg + "\n"))
}
logBuffer.Flush()
tg.Flush()
}()

// Send logs asynchronously
logChan <- "Async message 1"
logChan <- "Async message 2"
logChan <- "Async message 3"

close(logChan)
time.Sleep(1 * time.Second) // Wait for goroutine to finish
}
```

## 🔧 Advanced Configuration

### Custom Configuration

```go
cfg := logtg.TelegramConfig{
BotToken:     "your-bot-token",
DefaultGroup: -1001234567890,
GroupIDByType: map[string]int64{
"info":  -1001234567890,
"warn":  -1001234567890,
"error": -1002345678901, // Different group for errors
"debug": -1001234567890,
},
TopicIDByType: map[string]int64{
"info":  123,
"warn":  456,
"error": 789,
"debug": 012,
},
EnableAsync:   true,
FlushInterval: 10 * time.Second,
HTTPTimeout:   15 * time.Second,
BufferSize:    200,
}

logger := logtg.NewLogger(cfg)
```

### AutoFlush vs Manual Flush

```go
// AutoFlush: Messages are sent immediately
logger.SetupMultiWriter("error", true) // autoFlush = true
log.Println("This is sent immediately")

// Manual Flush: Messages are buffered
logger.SetupMultiWriter("info", false) // autoFlush = false
log.Println("This is buffered")
log.Println("This is also buffered")
logger.Flush() // Send all buffered messages
```

## 📊 Log Levels and Routing

The library supports four log levels:

- **INFO** - General information messages
- **WARN** - Warning messages
- **ERROR** - Error messages
- **DEBUG** - Debug information

Each level can be routed to:

- Different Telegram groups
- Different topics within the same group
- Different local outputs (console, files, etc.)

## 🎨 Terminal Output

The library provides colorized terminal output:

- 🟢 **INFO** - Green
- 🟡 **WARN** - Yellow
- 🔴 **ERROR** - Red
- 🔵 **DEBUG** - Cyan

## ⚡ Performance Features

- **Batching**: Messages are grouped and sent in batches
- **Chunking**: Long messages are automatically split
- **Concurrent sending**: Multiple message chunks sent concurrently
- **Connection pooling**: Reuses HTTP connections
- **Buffer management**: Efficient memory usage with pre-allocated buffers

## 🛡️ Error Handling

- **Retry mechanism**: Automatic retry with exponential backoff
- **Graceful degradation**: Continues working even if Telegram is unavailable
- **Context cancellation**: Proper cleanup on application shutdown
- **Rate limiting**: Prevents API rate limit violations

## 📋 API Reference

### Logger Methods

#### Traditional API

```go
logger.Info(format string, v ...interface{})
logger.Warn(format string, v ...interface{})
logger.Error(format string, v ...interface{})
logger.Debug(format string, v ...interface{})
logger.OnlyInfo(format string, v ...interface{}) // Console only
logger.ErrorWithError(err error, format string, v ...interface{})
```

#### MultiWriter API

```go
logger.SetupMultiWriter(logType string, autoFlush bool)
logger.SetupMultiWriterWithCustomOutput(outputs []io.Writer, logType string, autoFlush bool)
logger.GetLogBuffer(logType string, autoFlush bool) *LogBuffer
```

#### Control Methods

```go
logger.Flush() // Send all buffered messages
logger.Close() error // Graceful shutdown
```

### LogBuffer Methods

```go
logBuffer.Write(p []byte) (n int, err error) // io.Writer interface
logBuffer.Flush() // Flush buffer content
```

## 🐛 Troubleshooting

### Common Issues

1. **Bot token not working**
    - Verify the token with [@BotFather](https://t.me/botfather)
    - Check environment variables

2. **Messages not appearing in Telegram**
    - Ensure bot is added to the group/channel
    - Verify bot has admin privileges
    - Check group/topic IDs

3. **Performance issues**
    - Enable async mode: `EnableAsync: true`
    - Increase flush interval
    - Use autoFlush sparingly

4. **Memory usage**
    - Adjust buffer size in config
    - Call `Flush()` regularly
    - Monitor buffer growth

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## 📞 Support

If you have any questions or issues, please open an issue on GitHub or contact the maintainers.

---

**Happy Logging! 🚀**