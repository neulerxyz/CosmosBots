package telegram

import (
    "fmt"
    "log"
	"net/http"
    "strconv"
    "strings"
    "regexp"
    "encoding/json"
    "io"
    
	"github.com/neulerxyz/CosmosBots/config"

    tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type TelegramBot struct {
	cfg 			    *config.Config
	botApi  		    *tgbotapi.BotAPI
	commands 		    map[string]CommandInfo
	stop                chan struct{}
	missedBlocksCh      chan config.MissedBlocksEvent
    validatorDownCh     chan config.ValidatorDownEvent
    validatorResolvedCh chan config.ValidatorResolvedEvent
}

type CommandInfo struct {
    Handler func(update tgbotapi.Update)
    Help    string
}

func NewTelegramBot(cfg *config.Config, 
			missedBlocksCh chan config.MissedBlocksEvent, 
			validatorDownCh chan config.ValidatorDownEvent, 
            validatorResolvedCh chan config.ValidatorResolvedEvent) (*TelegramBot, error) {
    bot, err := tgbotapi.NewBotAPI(cfg.TelegramBotToken)
    if err != nil {
        return nil, fmt.Errorf("failed to create Telegram bot: %v", err)
    }
    if err != nil {
        return nil, fmt.Errorf("failed to get chat ID: %v", err)
    }

    telegramBot := &TelegramBot{
		cfg: 	 		 cfg,
        botApi:     	 bot,
        commands: 		 make(map[string]CommandInfo),
		stop:            make(chan struct{}),
		missedBlocksCh:  missedBlocksCh,
		validatorDownCh: validatorDownCh,
    }

    telegramBot.initCommands()

    return telegramBot, nil
}

func (tgb *TelegramBot) Run() {
    go tgb.handleTelegramCommands()

    for {
        select {
        case event := <-tgb.missedBlocksCh:
            message := fmt.Sprintf("Validator %s missed %d consecutive blocks from Height %d!", event.ValidatorAddress, event.MissedCount, int64(event.LastSignedHeight+1))
            tgb.sendTelegramMessage(message)
        
		case event := <-tgb.validatorDownCh:
			message := fmt.Sprintf("URGENT!! Validator down from height %d!", event.ValidatorAddress, int64(event.LastSignedHeight+1))
			tgb.sendTelegramMessage(message)
            
        case event := <-tgb.validatorResolvedCh:
            message := fmt.Sprintf("Validator %s is back online and signed block at height %d.", event.ValidatorAddress, event.LastSignedHeight)
            tgb.sendTelegramMessage(message)
		
		case <-tgb.stop:            return
        }
    }
}

func (tgb *TelegramBot) Stop() {
    close(tgb.stop)
}

func (tgb *TelegramBot) initCommands() {
    tgb.commands = map[string]CommandInfo{
        "validator_addr": {
            Handler: tgb.handleModifyValidatorAddr,
            Help:    "Modify the validator address. Usage: /validator_addr <hex-address>",
        },
        "missed_blocks": {
            Handler: tgb.handleMissedAmount,
            Help:    "Set the threshold of missed blocks before sending alerts to TG. Usage: /missed_blocks <number>",
        },
        "start_bot": {
            Handler: tgb.handleStartBot,
            Help:    "Activate the bot in a group. Usage: /start_bot",
        },
        "help": {
            Handler: tgb.handleHelp,
            Help:    "Show available commands and their usage.",
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

    for update := range updates {
        if update.Message == nil || !update.Message.IsCommand() {
            continue
        }

        command := update.Message.Command()
        if info, ok := tgb.commands[command]; ok {
            info.Handler(update)
        } else {
			tgb.sendTelegramMessage("Unknown command.")
        }
    }
}

func (tgb *TelegramBot) handleModifyValidatorAddr(update tgbotapi.Update) {
    args := update.Message.CommandArguments()

    if len(args) == 0 {
        tgb.sendTelegramMessage("Please provide a validator address.")
        return
    }

    validatorAddr := strings.TrimSpace(args)

    // Validate the provided validator address
    validationErr := tgb.isValidValidatorAddress(validatorAddr)
    if validationErr != nil {
		tgb.sendTelegramMessage(validationErr.Error())
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

func (tgb *TelegramBot) handleMissedAmount(update tgbotapi.Update) {
    args := update.Message.CommandArguments()    
    msg := ""

    if len(args) == 0 {
        msg = "Please provide the number of missed blocks."
    } else {
        missedThreshold, err := strconv.ParseInt(args, 10, 64)
        if err != nil {
            msg = "Invalid number of missed blocks. Please provide a valid integer."
        } else {
			tgb.cfg.SetMissedThreshold(missedThreshold)
            msg = fmt.Sprintf("Number of missed blocks updated to: %d", missedThreshold)
        }
    }
	tgb.sendTelegramMessage(msg)
}

func (tgb *TelegramBot) handleStartBot(update tgbotapi.Update) {
    chatID := update.Message.Chat.ID
    chatType := update.Message.Chat.Type

    if chatType == "group" || chatType == "supergroup" {
        tgb.cfg.SetTelegramChatID(chatID)
        tgb.sendTelegramMessage("Bot activated. Alerts will be sent to this group.")
    } else {
        tgb.sendTelegramMessage("Please add the bot to a group and use /start_bot command.")
    }
}

func (tgb *TelegramBot) handleHelp(update tgbotapi.Update) {
    commandsInfo := tgb.GetCommandsInfo()

    var helpText string
    for command, info := range commandsInfo {
        helpText += fmt.Sprintf("/%s - %s\n", command, info)
    }
	tgb.sendTelegramMessage(helpText)
}

func (tgb *TelegramBot) sendTelegramMessage(message string) {
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
// Getters

func (tgb *TelegramBot) GetCommandsInfo() map[string]string {
    commandsInfo := make(map[string]string)
    for command, info := range tgb.commands {
        commandsInfo[command] = info.Help
    }
    return commandsInfo
}