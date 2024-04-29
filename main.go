package main

import (
    "fmt"
    "log"

    "github.com/neulerxyz/CosmosBots/config"
    "github.com/neulerxyz/CosmosBots/bot"
    "github.com/neulerxyz/CosmosBots/telegram"
)

func main() {
    cfg, err := config.LoadConfig()
    if err != nil {
        log.Fatalf("Failed to load configuration: %v", err)
    }

    missedBlocksCh := make(chan config.MissedBlocksEvent)
    validatorDownCh := make(chan config.ValidatorDownEvent)
    validatorResCh := make(chan config.ValidatorResolvedEvent)
    Bot := bot.NewBot(cfg, missedBlocksCh, validatorDownCh, validatorResCh)
    telegramBot, err := telegram.NewTelegramBot(cfg, missedBlocksCh, validatorDownCh, validatorResCh)
    if err != nil {
        log.Fatalf("Failed to create TelegramBot: %v", err)
    }

    // Start Bot in a separate goroutine
    go func() {
        err := Bot.Start()
        if err != nil {
            log.Fatalf("Failed to start Bot: %v", err)
        }
        }()

    // Start TelegramBot in a separate goroutine
    go func() {
        telegramBot.Run()
    }()

    fmt.Println("Bot and TelegramBot started successfully!")

    // Wait indefinitely
    select {}
}