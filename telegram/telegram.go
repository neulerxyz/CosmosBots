package telegram

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"encoding/json"
	"io"
	"time"

	"github.com/neulerxyz/CosmosBots/config"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type TelegramBot struct {
	cfg                *config.Config
	botApi             *tgbotapi.BotAPI
	commands           map[string]CommandInfo
	stop               chan struct{}
	missedBlocksCh     chan config.MissedBlocksEvent
	validatorDownCh    chan config.ValidatorDownEvent
	validatorResolvedCh chan config.ValidatorResolvedEvent
	alertCh            chan string
	rateLimiter        <-chan time.Time
}

type CommandInfo struct {
	Handler func(update tgbotapi.Update, args string)
	Help    string
}

func NewTelegramBot(cfg *config.Config,
	missedBlocksCh chan config.MissedBlocksEvent,
	validatorDownCh chan config.ValidatorDownEvent,
	validatorResolvedCh chan config.ValidatorResolvedEvent,
	alertCh chan string) (*TelegramBot, error) {
	bot, err := tgbotapi.NewBotAPI(cfg.TelegramBotToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create Telegram bot: %v", err)
	}

	telegramBot := &TelegramBot{
		cfg:                cfg,
		botApi:             bot,
		commands:           make(map[string]CommandInfo),
		stop:               make(chan struct{}),
		missedBlocksCh:     missedBlocksCh,
		validatorDownCh:    validatorDownCh,
		validatorResolvedCh: validatorResolvedCh,
		alertCh:            alertCh,
		rateLimiter:        time.Tick(1 * time.Second), // Limit to 1 message per second
	}

	telegramBot.initCommands()

	return telegramBot, nil
}

func (tgb *TelegramBot) Run() {
	go tgb.handleTelegramCommands()

	for {
		select {
		case event := <-tgb.missedBlocksCh:
			message := fmt.Sprintf("Validator %s missed %d consecutive blocks from height %d!", event.ValidatorAddress, event.MissedCount, int64(event.LastSignedHeight+1))
			tgb.sendTelegramMessage(message)

		case event := <-tgb.validatorDownCh:
			message := fmt.Sprintf("URGENT!! Validator %s down for %d blocks from height %d!", event.ValidatorAddress, event.MissedCount, int64(event.LastSignedHeight+1))
            tgb.sendTelegramMessage(message)

		case event := <-tgb.validatorResolvedCh:
			message := fmt.Sprintf("Validator %s is back online and signed block at height %d.", event.ValidatorAddress, event.LastSignedHeight)
			tgb.sendTelegramMessage(message)

        case alert := <-tgb.alertCh:
			tgb.sendTelegramMessage(alert)    

		case <-tgb.stop:
			return
		}
	}
}

func (tgb *TelegramBot) Stop() {
	close(tgb.stop)
}

func (tgb *TelegramBot) initCommands() {
	prefix := strings.ToLower(tgb.cfg.Name)
	tgb.commands = map[string]CommandInfo{
		"validator_addr": {
			Handler: tgb.handleModifyValidatorAddr,
			Help:    "Modify the validator address. Usage: /" + prefix + " validator_addr <hex-address>",
		},
		"missed_blocks": {
			Handler: tgb.handleMissedAmount,
			Help:    "Set the threshold of missed blocks before sending alerts to TG. Usage: /" + prefix + " missed_blocks <number>",
		},
		"start_bot": {
			Handler: tgb.handleStartBot,
			Help:    "Activate the bot in a group. Usage: /" + prefix + " start_bot",
		},
		"faulty_endpoints": {
			Handler: tgb.handleFaultyEndpoints,
			Help:    "Show the list of faulty endpoints. Usage: /" + prefix + " faulty_endpoints",
		},
		"validator_status": {
			Handler: tgb.handleValidatorStatus,
			Help:    "Show the validator status. Usage: /" + prefix + " validator_status",
		},
		"node_status": {
			Handler: tgb.handleNodeStatus,
			Help:    "Show the node status. Usage: /" + prefix + " node_status",
		},
		"help": {
			Handler: tgb.handleHelp,
			Help:    "Show available commands and their usage. Usage: /" + prefix + " help",
		},
	}
}

func (tgb *TelegramBot) handleTelegramCommands() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := tgb.botApi.GetUpdatesChan(u)
	if err != nil {
		log.Fatal(err)
	}

	prefix := strings.ToLower(tgb.cfg.Name) + " "

	for update := range updates {
		if update.Message == nil {
			continue
		}

		message := update.Message.Text
		if strings.HasPrefix(message, "/"+prefix) {
			commandWithArgs := strings.TrimPrefix(message, "/"+prefix)
			parts := strings.SplitN(commandWithArgs, " ", 2)
			command := parts[0]
			args := ""
			if len(parts) > 1 {
				args = parts[1]
			}

			if info, ok := tgb.commands[command]; ok {
				info.Handler(update, args)
			} else {
				tgb.sendTelegramMessage("Unknown command.")
			}
		}
	}
}

func (tgb *TelegramBot) handleModifyValidatorAddr(update tgbotapi.Update, args string) {
	if args == "" {
		tgb.sendTelegramMessage("Please provide a validator address.")
		return
	}

	validatorAddr := strings.TrimSpace(args)

	// Validate the provided validator address
	if err := tgb.isValidValidatorAddress(validatorAddr); err != nil {
		tgb.sendTelegramMessage(err.Error())
		return
	}
	tgb.cfg.SetValidatorAddress(validatorAddr)
	tgb.sendTelegramMessage(fmt.Sprintf("Validator address updated to: %s", validatorAddr))
}

func (tgb *TelegramBot) isValidValidatorAddress(address string) error {
	// Format check using regular expression
	validatorAddrRegex := regexp.MustCompile(`^[a-fA-F0-9]{40}$`)
	if !validatorAddrRegex.MatchString(address) {
		return fmt.Errorf("invalid validator address format")
	}

	// On-chain check using RPC endpoint
	validatorFound, err := tgb.checkValidatorExists(address)

	if err != nil {
		return fmt.Errorf("failed to check validator existence: %v", err)
	}
	if !validatorFound {
		return fmt.Errorf("validator address not found on-chain")
	}

	return nil
}

func (tgb *TelegramBot) checkValidatorExists(address string) (bool, error) {
	// Make an RPC request to the /validators endpoint
	resp, err := http.Get(fmt.Sprintf("%s/validators?page=1&per_page=150", tgb.cfg.RPCEndpoint))
	if err != nil {
		fmt.Printf("Error trying to retrieve validators: %s\n", err)
		return false, err
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Unexpected status code: %d\n", resp.StatusCode)
		return false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response body: %s\n", err)
		return false, err
	}

	// Parse the response JSON
	var result struct {
		Result struct {
			Validators []struct {
				Address string `json:"address"`
			} `json:"validators"`
		} `json:"result"`
	}
	err = json.Unmarshal(body, &result)
	if err != nil {
		fmt.Printf("Error parsing JSON: %s\n", err)
		return false, err
	}
	// Search for the validator address in the response
	for _, validator := range result.Result.Validators {
		if validator.Address == address {
			return true, nil
		}
	}

	return false, nil
}

func (tgb *TelegramBot) handleMissedAmount(update tgbotapi.Update, args string) {
	if args == "" {
		tgb.sendTelegramMessage("Please provide the number of missed blocks.")
		return
	}

	missedThreshold, err := strconv.ParseInt(args, 10, 64)
	if err != nil {
		tgb.sendTelegramMessage("Invalid number of missed blocks. Please provide a valid integer.")
		return
	}

	tgb.cfg.SetMissedThreshold(missedThreshold)
	tgb.sendTelegramMessage(fmt.Sprintf("Number of missed blocks updated to: %d", missedThreshold))
}

func (tgb *TelegramBot) handleStartBot(update tgbotapi.Update, args string) {
	chatID := update.Message.Chat.ID
	chatType := update.Message.Chat.Type

	if chatType == "group" || chatType == "supergroup" {
		tgb.cfg.SetTelegramChatID(chatID)
		tgb.sendTelegramMessage("Bot activated. Alerts will be sent to this group.")
	} else {
		tgb.sendTelegramMessage("Please add the bot to a group and use /start_bot command.")
	}
}

func (tgb *TelegramBot) handleFaultyEndpoints(update tgbotapi.Update, args string) {
	faultyReferences := tgb.cfg.GetFaultyReferenceEndpoints()
	faultyChecks := tgb.cfg.GetFaultyCheckEndpoints()

	message := "Faulty Reference Endpoints:\n"
	for _, endpoint := range faultyReferences {
		message += fmt.Sprintf("- %s\n", endpoint)
	}

	message += "\nFaulty Check Endpoints:\n"
	for _, endpoint := range faultyChecks {
		message += fmt.Sprintf("- %s\n", endpoint)
	}

	tgb.sendTelegramMessage(message)
}

func (tgb *TelegramBot) handleValidatorStatus(update tgbotapi.Update, args string) {
	validatorAddress := tgb.cfg.GetValidatorAddress()
	lastSignedHeight := tgb.cfg.ValidatorStatus.LastSignedHeight
	isDown := tgb.cfg.ValidatorStatus.IsDown

	message := fmt.Sprintf("Validator Address: %s\nLast Signed Height: %d\nIs Down: %v", validatorAddress, lastSignedHeight, isDown)
	tgb.sendTelegramMessage(message)
}

func (tgb *TelegramBot) handleNodeStatus(update tgbotapi.Update, args string) {
	for _, endpoint := range tgb.cfg.GetCheckEndpoints() {
		status, exists := tgb.cfg.GetNodeStatus(endpoint)
		if !exists {
			message := fmt.Sprintf("No status available for endpoint: %s", endpoint)
			tgb.sendTelegramMessage(message)
			continue
		}

		message := fmt.Sprintf("Node %s\nLatest Block Height: %d\nIs Synced: %v", endpoint, status.LatestBlockHeight, status.IsSynced)
		tgb.sendTelegramMessage(message)
	}
}

func (tgb *TelegramBot) handleHelp(update tgbotapi.Update, args string) {
	commandsInfo := tgb.GetCommandsInfo()

	var helpText string
	for command, info := range commandsInfo {
		helpText += fmt.Sprintf("/%s %s - %s\n", strings.ToLower(tgb.cfg.Name), command, info)
	}
	tgb.sendTelegramMessage(helpText)
}

func (tgb *TelegramBot) sendTelegramMessage(message string) {
	<-tgb.rateLimiter // Ensure rate limiting
	chatID, err := strconv.ParseInt(tgb.cfg.GetTelegramChatID(), 10, 64)
	if err != nil {
		log.Printf("Invalid chat ID in configuration: %v", err)
		return
	}
	msg := tgbotapi.NewMessage(chatID, message)
	_, err = tgb.botApi.Send(msg)
	if err != nil {
		log.Printf("Failed to send Telegram message: %v", err)
	}
}

// GetCommandsInfo returns the help information for all available commands.
func (tgb *TelegramBot) GetCommandsInfo() map[string]string {
	commandsInfo := make(map[string]string)
	for command, info := range tgb.commands {
		commandsInfo[command] = info.Help
	}
	return commandsInfo
}
