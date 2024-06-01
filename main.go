package main

import (
	"fmt"
	"log"

	"github.com/neulerxyz/CosmosBots/config"
	"github.com/neulerxyz/CosmosBots/validatorbot"
	"github.com/neulerxyz/CosmosBots/nodebot"
	"github.com/neulerxyz/CosmosBots/telegram"
)

func main() {
	cfg, err := config.LoadConfig("config.toml")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	missedBlocksCh := make(chan config.MissedBlocksEvent)
	validatorDownCh := make(chan config.ValidatorDownEvent)
	validatorResCh := make(chan config.ValidatorResolvedEvent)
	alertCh := make(chan string)

	validatorBot := validatorbot.NewValidatorBot(cfg, missedBlocksCh, validatorDownCh, validatorResCh)
	telegramBot, err := telegram.NewTelegramBot(cfg, missedBlocksCh, validatorDownCh, validatorResCh)
	nodeBot := nodebot.NewNodeBot(cfg, alertCh)

	// Start Bots in a separate goroutine
	go func() {
		validatorBot.Start()
	}()
	go func() {
		telegramBot.Run()
	}()
	go func() {
		nodeBot.Start()
	}()

	fmt.Println("ValidatorBot, TelegramBot, and NodeBot started successfully!")

	// Wait indefinitely
	select {}
}
