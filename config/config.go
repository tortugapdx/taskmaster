package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	TelegramBotToken string `json:"telegram_bot_token"`
	TelegramUserID   int64  `json:"telegram_user_id"`
}

func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "taskmaster", "config.json")
}

func LoadFromFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) SaveToFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func (c Config) IsComplete() bool {
	return c.TelegramBotToken != "" && c.TelegramUserID != 0
}

func InteractiveSetup(r *bufio.Reader) (Config, error) {
	fmt.Println("Welcome to Taskmaster!")
	fmt.Println()

	fmt.Print("Enter your Telegram Bot Token (from @BotFather): ")
	token, err := r.ReadString('\n')
	if err != nil {
		return Config{}, err
	}
	token = strings.TrimSpace(token)

	fmt.Print("Enter your Telegram User ID: ")
	idStr, err := r.ReadString('\n')
	if err != nil {
		return Config{}, err
	}
	idStr = strings.TrimSpace(idStr)

	userID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return Config{}, fmt.Errorf("invalid user ID: %w", err)
	}

	return Config{
		TelegramBotToken: token,
		TelegramUserID:   userID,
	}, nil
}
