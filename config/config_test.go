package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`{"telegram_bot_token":"tok123","telegram_user_id":42}`), 0600)

	cfg, err := LoadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TelegramBotToken != "tok123" {
		t.Errorf("got token %q, want %q", cfg.TelegramBotToken, "tok123")
	}
	if cfg.TelegramUserID != 42 {
		t.Errorf("got user ID %d, want %d", cfg.TelegramUserID, 42)
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	_, err := LoadFromFile("/nonexistent/config.json")
	if err == nil {
		t.Fatal("expected error for missing config")
	}
}

func TestConfig_IsComplete(t *testing.T) {
	if (Config{}).IsComplete() {
		t.Error("empty config should not be complete")
	}
	if (Config{TelegramBotToken: "tok"}).IsComplete() {
		t.Error("config with only token should not be complete")
	}
	if (Config{TelegramUserID: 1}).IsComplete() {
		t.Error("config with only user ID should not be complete")
	}
	if !(Config{TelegramBotToken: "tok", TelegramUserID: 1}).IsComplete() {
		t.Error("full config should be complete")
	}
}

func TestSaveConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := Config{TelegramBotToken: "tok456", TelegramUserID: 99}
	if err := cfg.SaveToFile(path); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.TelegramBotToken != "tok456" || loaded.TelegramUserID != 99 {
		t.Errorf("round-trip failed: got %+v", loaded)
	}

	// Check file permissions
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Errorf("config file perm = %o, want 0600", info.Mode().Perm())
	}
}
