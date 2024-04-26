// config/config.go
package config

import (
    "fmt"
    "os"
	"sync"
    "strconv"
	
    "github.com/joho/godotenv"
)

type Config struct {
    RPCEndpoint      string
    ValidatorAddress string
    TelegramBotToken string
    TelegramChatID   string
    MissedThreshold  int64
	mutex            sync.RWMutex
}


func (c *Config) SetValidatorAddress(address string) {
    c.mutex.Lock()
    defer c.mutex.Unlock()
    c.ValidatorAddress = address
}

func (c *Config) SetMissedThreshold(missedThreshold int64) {
    c.mutex.Lock()
    defer c.mutex.Unlock()
    c.MissedThreshold = missedThreshold
}

func (c *Config) GetMissedThreshold() int64 {
    c.mutex.RLock()
    defer c.mutex.RUnlock()
    return c.MissedThreshold
}

func (c *Config) GetValidatorAddress() string {
    c.mutex.RLock()
    defer c.mutex.RUnlock()
    return c.ValidatorAddress
}

func (c *Config) GetTelegramChatID() string {
    c.mutex.RLock()
    defer c.mutex.RUnlock()
    return c.TelegramChatID
}

func (c *Config) SetTelegramChatID(chatID int64) {
    c.mutex.Lock()
    defer c.mutex.Unlock()
    c.TelegramChatID = strconv.FormatInt(chatID, 10)
}

func LoadConfig() (*Config, error) {
    err := godotenv.Load()
    if err != nil {
        return nil, fmt.Errorf("failed to load .env file: %v", err)
    }

    rpcEndpoint := os.Getenv("RPC_ENDPOINT")
    if rpcEndpoint == "" {
        return nil, fmt.Errorf("RPC_ENDPOINT not set in .env file")
    }

    validatorAddress := os.Getenv("VALIDATOR_ADDRESS")
    if validatorAddress == "" {
        return nil, fmt.Errorf("VALIDATOR_ADDRESS not set in .env file")
    }

    telegramBotToken := os.Getenv("TELEGRAM_BOT_TOKEN")
    if telegramBotToken == "" {
        return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN not set in .env file")
    }

    telegramChatID := os.Getenv("TELEGRAM_CHAT_ID")
    if telegramChatID == "" {
        return nil, fmt.Errorf("TELEGRAM_CHAT_ID not set in .env file")
    }

    missedThresholdStr := os.Getenv("MISSED_THRESHOLD")
    if missedThresholdStr == "" {
        return nil, fmt.Errorf("MISSED_THRESHOLD not set in .env file")
    }

    missedThreshold, err := strconv.ParseInt(missedThresholdStr, 10, 64)
    if err != nil {
        return nil, fmt.Errorf("invalid MISSED_THRESHOLD value: %v", err)
    }

    return &Config{
        RPCEndpoint:      rpcEndpoint,
        ValidatorAddress: validatorAddress,
        TelegramBotToken: telegramBotToken,
        TelegramChatID:   telegramChatID,
        MissedThreshold:  missedThreshold,
    }, nil
}