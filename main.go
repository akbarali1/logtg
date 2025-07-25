package logtg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

type TelegramConfig struct {
	BotToken      string
	DefaultGroup  int64
	GroupIDByType map[string]int64 // logType -> groupId
	TopicIDByType map[string]int64 // logType -> topicId
}

type Logger struct {
	telegram TelegramConfig
	buffers  map[string][]string // logType -> messages
	mu       sync.Mutex
}

func IsRunningInDocker() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	return false
}

func init() {
	// Setup logging
	// Load environment variables
	if !IsRunningInDocker() {
		_, filename, _, _ := runtime.Caller(0)
		envFile := filepath.Dir(filename) + "/.env"
		if err := godotenv.Load(envFile); err != nil {
			println(envFile)
			println(err.Error())
			log.Fatal("Error loading .env file")
		}
	}
}

func InitLog() *Logger {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		logrus.Fatal("TELEGRAM_BOT_TOKEN is not set")
	}
	groupStr := os.Getenv("TELEGRAM_GROUP_ID")
	if groupStr == "" {
		logrus.Fatal("TELEGRAM_GROUP_ID is not set")
	}
	groupID, err := strconv.ParseInt(groupStr, 10, 64)
	if err != nil {
		logrus.WithError(err).Fatal("TELEGRAM_GROUP_ID is not set")
		logrus.Fatal("TELEGRAM_GROUP_ID is not set")
	}

	infoTopicStr := os.Getenv("TELEGRAM_GROUP_INFO_TOPIC_ID")
	if infoTopicStr == "" {
		logrus.Fatal("TELEGRAM_GROUP_INFO_TOPIC_ID is not set")
	}
	infoTopicID, err := strconv.ParseInt(infoTopicStr, 10, 64)
	if err != nil {
		logrus.WithError(err).Fatal("TELEGRAM_GROUP_INFO_TOPIC_ID is not set")
		logrus.Fatal("TELEGRAM_GROUP_INFO_TOPIC_ID is not set")
	}

	errorTopicStr := os.Getenv("TELEGRAM_GROUP_ERROR_TOPIC_ID")
	if errorTopicStr == "" {
		logrus.Fatal("TELEGRAM_GROUP_ERROR_TOPIC_ID is not set")
	}
	errorTopicID, err := strconv.ParseInt(errorTopicStr, 10, 64)
	if err != nil {
		logrus.WithError(err).Fatal("TELEGRAM_GROUP_ERROR_TOPIC_ID is not set")
		logrus.Fatal("TELEGRAM_GROUP_ERROR_TOPIC_ID is not set")
	}

	warnTopicStr := os.Getenv("TELEGRAM_GROUP_WARN_TOPIC_ID")
	warnTopicID, err := strconv.ParseInt(warnTopicStr, 10, 64)
	if err != nil {
		logrus.Warn("TELEGRAM_GROUP_WARN_TOPIC_ID is not set" + err.Error())
		warnTopicID = 0
	}

	debugTopicStr := os.Getenv("TELEGRAM_GROUP_DEBUG_TOPIC_ID")
	debugTopicID, err := strconv.ParseInt(debugTopicStr, 10, 64)
	if err != nil {
		logrus.Warn("TELEGRAM_GROUP_DEBUG_TOPIC_ID is not set: " + err.Error())
		debugTopicID = 0
	}

	return NewLogger(TelegramConfig{
		BotToken:     botToken,
		DefaultGroup: groupID,
		GroupIDByType: map[string]int64{
			"info":  groupID,
			"warn":  groupID,
			"error": groupID,
			"debug": groupID,
		},
		TopicIDByType: map[string]int64{
			"info":  infoTopicID,
			"warn":  warnTopicID,
			"error": errorTopicID,
			"debug": debugTopicID,
		},
	})
}

// Create a new logger instance
func NewLogger(cfg TelegramConfig) *Logger {
	l := &Logger{
		telegram: cfg,
		buffers:  make(map[string][]string),
	}
	// Automatic flush when the program terminates
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		<-c
		l.Flush()
		os.Exit(0)
	}()
	return l
}

// Internal log function: prints to stdout and adds to buffer
func (l *Logger) log(logType, format string, v ...interface{}) {
	noLog := false
	if logType == "only_info" {
		noLog = true
		logType = "info"
	}

	msg := fmt.Sprintf(format, v...)
	timestamp := time.Now().Format("15:04:05 02.01.2006")
	fullMsg := fmt.Sprintf("%s [%s] %s", timestamp, strings.ToUpper(logType), msg)

	// Print to terminal, with color for info
	if logType == "info" {
		fmt.Printf("\033[32m %s \033[0m\n", fullMsg)
	} else {
		fmt.Println(fullMsg)
	}
	if noLog {
		return // Skip sending to Telegram if disabled
	}

	// Add to buffer for Telegram
	l.mu.Lock()
	l.buffers[logType] = append(l.buffers[logType], fullMsg)
	l.mu.Unlock()
}

// ErrorWithError: log error with err and message, like logrus.WithError(err).Error("message")
func (l *Logger) ErrorWithError(err error, format string, v ...interface{}) {
	// Message with error appended
	userMsg := fmt.Sprintf(format, v...)
	fullMsg := fmt.Sprintf("%s | error: %v", userMsg, err)
	// Print to terminal
	logrus.WithError(err).Error(userMsg)
	// Add to buffer (type "error")
	timestamp := time.Now().Format("15:04:05 02.01.2006")
	finalMsg := fmt.Sprintf("%s [ERROR] %s", timestamp, fullMsg)
	println(finalMsg)
	l.mu.Lock()
	l.buffers["error"] = append(l.buffers["error"], finalMsg)
	l.mu.Unlock()
}

// Public log methods
func (l *Logger) Info(format string, v ...interface{})     { l.log("info", format, v...) }
func (l *Logger) OnlyInfo(format string, v ...interface{}) { l.log("only_info", format, v...) }
func (l *Logger) Warn(format string, v ...interface{})     { l.log("warn", format, v...) }
func (l *Logger) Error(format string, v ...interface{})    { l.log("error", format, v...) }
func (l *Logger) Debug(format string, v ...interface{})    { l.log("debug", format, v...) }

// Flush: sends all buffered logs to Telegram
func (l *Logger) Flush() {
	l.mu.Lock()
	defer l.mu.Unlock()
	var wg sync.WaitGroup
	const maxTelegramLen = 3900 // 4000 + <pre>...</pre> or Markdown tag, safe margin

	for logType, messages := range l.buffers {
		if len(messages) == 0 {
			continue
		}
		groupID := l.telegram.GroupIDByType[logType]
		if groupID == 0 {
			groupID = l.telegram.DefaultGroup
		}
		topicID := l.telegram.TopicIDByType[logType]

		var chunk []string
		currentLen := 0

		sendChunk := func(msgs []string) {
			wg.Add(1)
			text := strings.Join(msgs, "\n")
			payload := map[string]interface{}{
				"chat_id": groupID,
				"text":    text,
			}
			if topicID != 0 {
				payload["message_thread_id"] = topicID
			}
			sendToTelegram(l.telegram.BotToken, payload, &wg)
		}

		for _, msg := range messages {
			// +1 for the '\n' separator except the first line
			addLen := len(msg)
			if len(chunk) > 0 {
				addLen++ // for the newline
			}
			if currentLen+addLen > maxTelegramLen {
				// Send previous chunk, start new one
				if len(chunk) > 0 {
					sendChunk(chunk)
				}
				chunk = []string{msg}
				currentLen = len(msg)
			} else {
				chunk = append(chunk, msg)
				currentLen += addLen
			}
		}
		if len(chunk) > 0 {
			sendChunk(chunk)
		}
		// Clear the buffer
		l.buffers[logType] = nil
	}
	wg.Wait()
}

func sendToTelegram(botToken string, payload map[string]interface{}, wg *sync.WaitGroup) {
	defer wg.Done()
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	// Insert double line-break between log lines for readability
	if text, ok := payload["text"].(string); ok {
		// Split by lines, add bullet, then join back
		lines := strings.Split(text, "\n")
		for i, line := range lines {
			if len(strings.TrimSpace(line)) > 0 {
				lines[i] = "• " + line
			}
		}
		newText := strings.Join(lines, "\n")
		payload["text"] = "<pre>" + htmlEscape(newText) + "</pre>"
	}
	payload["parse_mode"] = "HTML"

	jsonData, _ := json.Marshal(payload)
	res, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		logrus.WithError(err).Error("Failed to send message to Telegram")
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			logrus.WithError(err).Error("Failed to close Telegram response body")
		}
	}(res.Body)
	if res.StatusCode != http.StatusOK {
		msgBody := fmt.Sprintf("Telegram API returned status %d", res.StatusCode)
		body, _ := io.ReadAll(res.Body)
		msgBody += fmt.Sprintf("\nRequest payload: %s", string(jsonData))
		msgBody += fmt.Sprintf("\nResponse: %s", body)
		logrus.Errorf(msgBody)
		return
	}
}

// HTML escape for safe Telegram <pre> block
func htmlEscape(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	)
	return replacer.Replace(s)
}
