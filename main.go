package main

import (
	"bufio"
	"fmt"
	"log"
	"os"

	"github.com/jpoz/taskmaster/bot"
	"github.com/jpoz/taskmaster/config"
)

func main() {
	cfgPath := config.DefaultPath()

	cfg, err := config.LoadFromFile(cfgPath)
	if err != nil || !cfg.IsComplete() {
		reader := bufio.NewReader(os.Stdin)
		cfg, err = config.InteractiveSetup(reader)
		if err != nil {
			log.Fatalf("Setup failed: %v", err)
		}
		if err := cfg.SaveToFile(cfgPath); err != nil {
			log.Fatalf("Failed to save config: %v", err)
		}
		fmt.Printf("\nConfig saved to %s\n", cfgPath)
	}

	fmt.Println("Starting bot...")

	b, err := bot.New(cfg.TelegramBotToken, cfg.TelegramUserID)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	if err := b.Run(); err != nil {
		log.Fatalf("Bot error: %v", err)
	}
}
