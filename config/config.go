package config

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"github.com/pelletier/go-toml"
)

type NodeStatus struct {
	LatestBlockHeight int
	IsSynced          bool
}

type ValidatorStatus struct {
	LastSignedHeight int64
	IsDown           bool
}

type Config struct {
	Name                     string   `toml:"name"`
	RPCEndpoint              string   `toml:"rpcEndpoint"`
	ValidatorAddress         string   `toml:"validatorAddress"`
	TelegramBotToken         string   `toml:"telegramBotToken"`
	TelegramChatID           string   `toml:"telegramChatID"`
	MissedThreshold          int64    `toml:"missedThreshold"`
	RepeatThreshold          int64    `toml:"repeatThreshold"`
	ReferenceEndpoints       []string `toml:"referenceEndpoints"`
	CheckEndpoints           []string `toml:"checkEndpoints"`
	FaultyReferenceEndpoints map[string]bool
	FaultyCheckEndpoints     map[string]bool
	NodeStatuses             map[string]NodeStatus
	ValidatorStatus          ValidatorStatus
	Mutex                    sync.RWMutex // Exported mutex field
}

func (c *Config) SetValidatorAddress(address string) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	c.ValidatorAddress = address
}

func (c *Config) GetValidatorAddress() string {
	c.Mutex.RLock()
	defer c.Mutex.RUnlock()
	return c.ValidatorAddress
}

func (c *Config) SetTelegramChatID(chatID int64) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	c.TelegramChatID = strconv.FormatInt(chatID, 10)
}

func (c *Config) GetTelegramChatID() string {
	c.Mutex.RLock()
	defer c.Mutex.RUnlock()
	return c.TelegramChatID
}

func (c *Config) SetMissedThreshold(missedThreshold int64) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	c.MissedThreshold = missedThreshold
}

func (c *Config) GetMissedThreshold() int64 {
	c.Mutex.RLock()
	defer c.Mutex.RUnlock()
	return c.MissedThreshold
}

func (c *Config) GetRepeatThreshold() int64 {
	c.Mutex.RLock()
	defer c.Mutex.RUnlock()
	return c.RepeatThreshold
}

func (c *Config) SetReferenceEndpoints(endpoints []string) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	c.ReferenceEndpoints = endpoints
}

func (c *Config) GetReferenceEndpoints() []string {
	c.Mutex.RLock()
	defer c.Mutex.RUnlock()
	return c.ReferenceEndpoints
}

func (c *Config) SetCheckEndpoints(endpoints []string) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	c.CheckEndpoints = endpoints
}

func (c *Config) GetCheckEndpoints() []string {
	c.Mutex.RLock()
	defer c.Mutex.RUnlock()
	return c.CheckEndpoints
}

func (c *Config) AddFaultyReferenceEndpoint(endpoint string) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	if c.FaultyReferenceEndpoints == nil {
		c.FaultyReferenceEndpoints = make(map[string]bool)
	}
	c.FaultyReferenceEndpoints[endpoint] = true
}

func (c *Config) GetFaultyReferenceEndpoints() []string {
	c.Mutex.RLock()
	defer c.Mutex.RUnlock()
	keys := make([]string, 0, len(c.FaultyReferenceEndpoints))
	for key := range c.FaultyReferenceEndpoints {
		keys = append(keys, key)
	}
	return keys
}

func (c *Config) AddFaultyCheckEndpoint(endpoint string) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	if c.FaultyCheckEndpoints == nil {
		c.FaultyCheckEndpoints = make(map[string]bool)
	}
	c.FaultyCheckEndpoints[endpoint] = true
}

func (c *Config) GetFaultyCheckEndpoints() []string {
	c.Mutex.RLock()
	defer c.Mutex.RUnlock()
	keys := make([]string, 0, len(c.FaultyCheckEndpoints))
	for key := range c.FaultyCheckEndpoints {
		keys = append(keys, key)
	}
	return keys
}

func (c *Config) SetNodeStatus(endpoint string, status NodeStatus) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	if c.NodeStatuses == nil {
		c.NodeStatuses = make(map[string]NodeStatus)
	}
	c.NodeStatuses[endpoint] = status
}

func (c *Config) GetNodeStatus(endpoint string) (NodeStatus, bool) {
	c.Mutex.RLock()
	defer c.Mutex.RUnlock()
	status, exists := c.NodeStatuses[endpoint]
	return status, exists
}

func (c *Config) SetValidatorLastSignedHeight(height int64) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	c.ValidatorStatus.LastSignedHeight = height
}

func (c *Config) GetValidatorLastSignedHeight() int64 {
	c.Mutex.RLock()
	defer c.Mutex.RUnlock()
	return c.ValidatorStatus.LastSignedHeight
}

func (c *Config) SetValidatorIsDown(isDown bool) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	c.ValidatorStatus.IsDown = isDown
}

func (c *Config) GetValidatorIsDown() bool {
	c.Mutex.RLock()
	defer c.Mutex.RUnlock()
	return c.ValidatorStatus.IsDown
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
