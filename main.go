package logtg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

const (
	maxTelegramLen = 3900
	flushInterval  = 5 * time.Second
	maxRetries     = 3
	retryDelay     = 1 * time.Second
)

type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

type TelegramConfig struct {
	BotToken      string
	DefaultGroup  int64
	GroupIDByType map[string]int64
	TopicIDByType map[string]int64
	EnableAsync   bool // Async flush uchun
	FlushInterval time.Duration
	HTTPTimeout   time.Duration
	BufferSize    int // Buffer o'lchami
}

type Logger struct {
	telegram   TelegramConfig
	buffers    map[string][]string
	mu         sync.RWMutex // RWMutex ishlatish
	ctx        context.Context
	cancel     context.CancelFunc
	flushTimer *time.Timer
	httpClient *http.Client
	wg         sync.WaitGroup
	logPrint   bool
}

// LogBuffer - io.Writer interfaceini implement qiladi
type LogBuffer struct {
	logger    *Logger
	logType   string
	buffer    *bytes.Buffer
	mu        sync.Mutex
	autoFlush bool
}

func (l *Logger) NoLogPrint(typeLog bool) *Logger {
	l.logPrint = typeLog
	return l
}

// Write - io.Writer interface methodi
func (lb *LogBuffer) Write(p []byte) (n int, err error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	// Bufferga yozish
	n, err = lb.buffer.Write(p)
	if err != nil {
		return n, err
	}

	// Agar satr tugagan bo'lsa (newline bor), processlash
	if bytes.Contains(p, []byte("\n")) {
		lb.processBuffer()
	}

	return n, nil
}

// processBuffer - bufferdan satrlarni o'qib, telegram loggeriga yuboradi
func (lb *LogBuffer) processBuffer() {
	content := lb.buffer.String()
	if content == "" {
		return
	}

	lines := strings.Split(content, "\n")
	lb.buffer.Reset()

	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			// Timestamp qo'shish
			timestamp := time.Now().Format("15:04:05 02.01.2006")
			fullMsg := fmt.Sprintf("%s [%s] %s", timestamp, strings.ToUpper(lb.logType), trimmed)

			// Terminal output
			if lb.logger.logPrint {
				lb.printToTerminal(fullMsg)
			}

			// Telegram bufferiga qo'shish
			lb.logger.mu.Lock()
			if lb.logger.buffers[lb.logType] == nil {
				lb.logger.buffers[lb.logType] = make([]string, 0, lb.logger.telegram.BufferSize)
			}
			lb.logger.buffers[lb.logType] = append(lb.logger.buffers[lb.logType], fullMsg)
			lb.logger.mu.Unlock()

			// Auto flush
			if lb.autoFlush {
				go lb.logger.Flush()
			}
		}
	}
}

// printToTerminal - terminalga rangli output beradi
func (lb *LogBuffer) printToTerminal(msg string) {
	switch lb.logType {
	case "info":
		fmt.Printf("\033[32m%s\033[0m\n", msg)
	case "warn":
		fmt.Printf("\033[33m%s\033[0m\n", msg)
	case "error":
		fmt.Printf("\033[31m%s\033[0m\n", msg)
	case "debug":
		fmt.Printf("\033[36m%s\033[0m\n", msg)
	default:
		fmt.Println(msg)
	}
}

// Flush - LogBuffer bufferini tozalaydi
func (lb *LogBuffer) Flush() {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	if lb.buffer.Len() > 0 {
		lb.processBuffer()
	}
}

func IsRunningInDocker() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
}

func init() {
	envFile := ".env"
	if !IsRunningInDocker() {
		if err := godotenv.Load(envFile); err != nil {
			log.Printf("Warning: cannot load .env file from %s: %v\n", envFile, err)
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
		logrus.WithError(err).Fatal("Invalid TELEGRAM_GROUP_ID")
	}

	// Helper function to parse topic IDs
	parseTopicID := func(envVar string, required bool) int64 {
		str := os.Getenv(envVar)
		if str == "" {
			if required {
				logrus.Fatalf("%s is not set", envVar)
			}
			return 0
		}

		id, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			if required {
				logrus.WithError(err).Fatalf("Invalid %s", envVar)
			}
			logrus.Warnf("Invalid %s: %v", envVar, err)
			return 0
		}
		return id
	}

	infoTopicID := parseTopicID("TELEGRAM_GROUP_INFO_TOPIC_ID", true)
	errorTopicID := parseTopicID("TELEGRAM_GROUP_ERROR_TOPIC_ID", true)
	warnTopicID := parseTopicID("TELEGRAM_GROUP_WARN_TOPIC_ID", false)
	debugTopicID := parseTopicID("TELEGRAM_GROUP_DEBUG_TOPIC_ID", false)

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
		EnableAsync:   true,
		FlushInterval: flushInterval,
		HTTPTimeout:   10 * time.Second,
		BufferSize:    100,
	})
}

func NewLogger(cfg TelegramConfig) *Logger {
	ctx, cancel := context.WithCancel(context.Background())

	if cfg.FlushInterval == 0 {
		cfg.FlushInterval = flushInterval
	}
	if cfg.HTTPTimeout == 0 {
		cfg.HTTPTimeout = 10 * time.Second
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = 100
	}

	l := &Logger{
		telegram: cfg,
		buffers:  make(map[string][]string),
		ctx:      ctx,
		cancel:   cancel,
		httpClient: &http.Client{
			Timeout: cfg.HTTPTimeout,
		},
		logPrint: true,
	}

	// Graceful shutdown
	go l.handleShutdown()

	// Periodic flush if async enabled
	if cfg.EnableAsync {
		go l.periodicFlush()
	}

	return l
}

// GetLogBuffer - ma'lum log type uchun LogBuffer qaytaradi
func (l *Logger) GetLogBuffer(logType string, autoFlush bool) *LogBuffer {
	return &LogBuffer{
		logger:    l,
		logType:   logType,
		buffer:    &bytes.Buffer{},
		autoFlush: autoFlush,
	}
}

// SetupMultiWriter - standard log package uchun MultiWriter sozlaydi
func (l *Logger) SetupMultiWriter(logType string, autoFlush bool) {
	logBuffer := l.GetLogBuffer(logType, autoFlush)
	multiWriter := io.MultiWriter(os.Stderr, logBuffer)
	log.SetOutput(multiWriter)
}

// SetupMultiWriterWithCustomOutput - custom output bilan MultiWriter sozlaydi
func (l *Logger) SetupMultiWriterWithCustomOutput(outputs []io.Writer, logType string, autoFlush bool) {
	logBuffer := l.GetLogBuffer(logType, autoFlush)
	outputs = append(outputs, logBuffer)
	multiWriter := io.MultiWriter(outputs...)
	log.SetOutput(multiWriter)
}

func (l *Logger) handleShutdown() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	l.cancel()
	l.Flush()
	l.wg.Wait()
	os.Exit(0)
}

func (l *Logger) periodicFlush() {
	ticker := time.NewTicker(l.telegram.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.Flush()
		case <-l.ctx.Done():
			return
		}
	}
}

// Optimized log function with pre-formatting
func (l *Logger) log(logType, format string, v ...interface{}) {
	noLog := false
	if logType == "only_info" {
		noLog = true
		logType = "info"
	}

	msg := fmt.Sprintf(format, v...)
	timestamp := time.Now().Format("15:04:05 02.01.2006")
	fullMsg := fmt.Sprintf("%s [%s] %s", timestamp, strings.ToUpper(logType), msg)

	// Terminal output with colors
	switch logType {
	case "info":
		fmt.Printf("\033[32m%s\033[0m\n", fullMsg)
	case "warn":
		fmt.Printf("\033[33m%s\033[0m\n", fullMsg)
	case "error":
		fmt.Printf("\033[31m%s\033[0m\n", fullMsg)
	case "debug":
		fmt.Printf("\033[36m%s\033[0m\n", fullMsg)
	default:
		fmt.Println(fullMsg)
	}

	if noLog {
		return
	}

	// Buffer with read-write lock
	l.mu.Lock()
	if l.buffers[logType] == nil {
		l.buffers[logType] = make([]string, 0, l.telegram.BufferSize)
	}
	l.buffers[logType] = append(l.buffers[logType], fullMsg)
	l.mu.Unlock()
}

// Improved ErrorWithError method
func (l *Logger) ErrorWithError(err error, format string, v ...interface{}) {
	userMsg := fmt.Sprintf(format, v...)
	fullMsg := fmt.Sprintf("%s | error: %v", userMsg, err)

	logrus.WithError(err).Error(userMsg)

	timestamp := time.Now().Format("15:04:05 02.01.2006")
	finalMsg := fmt.Sprintf("%s [ERROR] %s", timestamp, fullMsg)

	l.mu.Lock()
	if l.buffers["error"] == nil {
		l.buffers["error"] = make([]string, 0, l.telegram.BufferSize)
	}
	l.buffers["error"] = append(l.buffers["error"], finalMsg)
	l.mu.Unlock()
}

// Public logging methods
func (l *Logger) Info(format string, v ...interface{})     { l.log("info", format, v...) }
func (l *Logger) OnlyInfo(format string, v ...interface{}) { l.log("only_info", format, v...) }
func (l *Logger) Warn(format string, v ...interface{})     { l.log("warn", format, v...) }
func (l *Logger) Error(format string, v ...interface{})    { l.log("error", format, v...) }
func (l *Logger) Debug(format string, v ...interface{})    { l.log("debug", format, v...) }

// Optimized Flush with batching and retry logic
func (l *Logger) Flush() {
	l.mu.Lock()
	if len(l.buffers) == 0 {
		l.mu.Unlock()
		return
	}

	// Create local copy and clear buffers
	localBuffers := make(map[string][]string)
	for logType, messages := range l.buffers {
		if len(messages) > 0 {
			localBuffers[logType] = make([]string, len(messages))
			copy(localBuffers[logType], messages)
			l.buffers[logType] = l.buffers[logType][:0] // Clear but keep capacity
		}
	}
	l.mu.Unlock()

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 5) // Limit concurrent requests

	for logType, messages := range localBuffers {
		if len(messages) == 0 {
			continue
		}

		groupID := l.telegram.GroupIDByType[logType]
		if groupID == 0 {
			groupID = l.telegram.DefaultGroup
		}
		topicID := l.telegram.TopicIDByType[logType]

		// Split messages into chunks
		chunks := l.splitIntoChunks(messages)

		for _, chunk := range chunks {
			wg.Add(1)
			go func(msgs []string, gID, tID int64) {
				defer wg.Done()
				semaphore <- struct{}{}        // Acquire
				defer func() { <-semaphore }() // Release

				l.sendChunkWithRetry(msgs, gID, tID)
			}(chunk, groupID, topicID)
		}
	}

	wg.Wait()
}

// Split messages into optimal chunks
func (l *Logger) splitIntoChunks(messages []string) [][]string {
	var chunks [][]string
	var currentChunk []string
	currentLen := 0

	for _, msg := range messages {
		addLen := len(msg)
		if len(currentChunk) > 0 {
			addLen++ // for newline
		}

		if currentLen+addLen > maxTelegramLen && len(currentChunk) > 0 {
			chunks = append(chunks, currentChunk)
			currentChunk = []string{msg}
			currentLen = len(msg)
		} else {
			currentChunk = append(currentChunk, msg)
			currentLen += addLen
		}
	}

	if len(currentChunk) > 0 {
		chunks = append(chunks, currentChunk)
	}

	return chunks
}

// Send chunk with retry logic
func (l *Logger) sendChunkWithRetry(messages []string, groupID, topicID int64) {
	text := strings.Join(messages, "\n")
	payload := map[string]interface{}{
		"chat_id":    groupID,
		"text":       l.formatForTelegram(text),
		"parse_mode": "HTML",
	}

	if topicID != 0 {
		payload["message_thread_id"] = topicID
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(retryDelay * time.Duration(attempt))
		}

		if err := l.sendToTelegram(payload); err != nil {
			lastErr = err
			continue
		}
		return // Success
	}

	logrus.WithError(lastErr).Error("Failed to send message to Telegram after retries")
}

// Optimized HTTP request
func (l *Logger) sendToTelegram(payload map[string]interface{}) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", l.telegram.BotToken)

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(l.ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API returned status %d: %s", resp.StatusCode, body)
	}

	return nil
}

// Optimized text formatting
func (l *Logger) formatForTelegram(text string) string {
	lines := strings.Split(text, "\n")
	result := make([]string, 0, len(lines))

	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			result = append(result, "• "+trimmed)
		}
	}

	return "<pre>" + htmlEscape(strings.Join(result, "\n")) + "</pre>"
}

// Optimized HTML escaping
func htmlEscape(s string) string {
	// Use strings.Builder for better performance
	var builder strings.Builder
	builder.Grow(len(s) * 2) // Pre-allocate some extra space

	for _, r := range s {
		switch r {
		case '&':
			builder.WriteString("&amp;")
		case '<':
			builder.WriteString("&lt;")
		case '>':
			builder.WriteString("&gt;")
		default:
			builder.WriteRune(r)
		}
	}

	return builder.String()
}

// Graceful shutdown method
func (l *Logger) Close() error {
	l.cancel()
	l.Flush()
	l.wg.Wait()
	return nil
}
