package main

import (
	"log"

	"github.com/neulerxyz/CosmosBots/config"
	"github.com/neulerxyz/CosmosBots/validatorbot"
	"github.com/neulerxyz/CosmosBots/telegram"
	"github.com/neulerxyz/CosmosBots/nodebot"
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

	// Initialize and start TelegramBot
	telegramBot, err := telegram.NewTelegramBot(cfg, missedBlocksCh, validatorDownCh, validatorResCh, alertCh)
	if err != nil {
		log.Fatalf("Failed to create TelegramBot: %v", err)
	}
	go telegramBot.Run()

	// Conditionally initialize and start ValidatorBot
	if cfg.GetValidatorAddress() != "" {
		validatorBot := validatorbot.NewValidatorBot(cfg, missedBlocksCh, validatorDownCh, validatorResCh)
		go func() {
			err := validatorBot.Start()
			if err != nil {
				log.Fatalf("Failed to start ValidatorBot: %v", err)
			}
		}()
	} else {
		log.Println("ValidatorBot not started as ValidatorAddress is not set.")
	}

	// Conditionally initialize and start NodeBot
	if len(cfg.GetCheckEndpoints()) > 0 {
		nodeBot := nodebot.NewNodeBot(cfg, alertCh)
		go func() {
			nodeBot.Start()
		}()
	} else {
		log.Println("NodeBot not started as CheckEndpoints is not set.")
	}

	log.Println("TelegramBot started successfully!")

	// Wait indefinitely
	select {}
}
