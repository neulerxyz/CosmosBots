package config

import (
	"fmt"
	"os"
	"sync"
    "strconv"
	"github.com/pelletier/go-toml"
)

type Config struct {
	RPCEndpoint       string   `toml:"rpcEndpoint"`
	ValidatorAddress  string   `toml:"validatorAddress"`
	TelegramBotToken  string   `toml:"telegramBotToken"`
	TelegramChatID    string   `toml:"telegramChatID"`
	MissedThreshold   int64    `toml:"missedThreshold"`
	RepeatThreshold   int64    `toml:"repeatThreshold"`
	ReferenceEndpoint string   `toml:"referenceEndpoint"`
	CheckEndpoints    []string `toml:"checkEndpoints"`
	mutex             sync.RWMutex
}

func (c *Config) SetValidatorAddress(address string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.ValidatorAddress = address
}

func (c *Config) GetValidatorAddress() string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.ValidatorAddress
}

func (c *Config) SetTelegramChatID(chatID int64) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.TelegramChatID = strconv.FormatInt(chatID, 10)
}

func (c *Config) GetTelegramChatID() string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.TelegramChatID
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

func (c *Config) GetRepeatThreshold() int64 {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.RepeatThreshold
}

func (c *Config) SetReferenceEndpoint(endpoint string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.ReferenceEndpoint = endpoint
}

func (c *Config) GetReferenceEndpoint() string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.ReferenceEndpoint
}

func (c *Config) SetCheckEndpoints(endpoints []string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.CheckEndpoints = endpoints
}

func (c *Config) GetCheckEndpoints() []string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.CheckEndpoints
}

func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %v", err)
	}
	defer file.Close()

	var cfg Config
	decoder := toml.NewDecoder(file)
	err = decoder.Decode(&cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to decode config file: %v", err)
	}

	return &cfg, nil
}
