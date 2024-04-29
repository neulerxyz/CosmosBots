package bot

import (
    "context"
    "log"
    
    "github.com/neulerxyz/CosmosBots/config"
	tmtypes "github.com/cometbft/cometbft/types"
    chttp "github.com/cometbft/cometbft/rpc/client/http"
    ctypes "github.com/cometbft/cometbft/rpc/core/types"
)


type Bot struct {
    cfg 			    *config.Config
	rpcEndpoint 	    string
    rpcClient           *chttp.HTTP
    missedBlocksCh      chan config.MissedBlocksEvent
    validatorDownCh     chan config.ValidatorDownEvent
    validatorResolvedCh chan config.ValidatorResolvedEvent
}

func NewBot(cfg *config.Config, 
        missedBlocksCh chan config.MissedBlocksEvent, 
        validatorDownCh chan config.ValidatorDownEvent,
        validatorResolvedCh chan config.ValidatorResolvedEvent ) *Bot {
    bot := &Bot{
        cfg: 			  cfg,
		rpcEndpoint:	  cfg.RPCEndpoint,
        missedBlocksCh:   missedBlocksCh,
        validatorDownCh:  validatorDownCh,
    }
    return bot
}

func (b *Bot) Start() error {
    var err error
    b.rpcClient, err = b.createRPCClient()
    if err != nil {
        log.Fatal(err)
    }

    newBlockEventCh, err := b.subscribeToEvent(b.rpcClient, "tm.event='NewBlock'", "newBlockSubscriber")
    if err != nil {
        log.Fatal(err)
    }

    newProposalEventCh, err := b.subscribeToEvent(b.rpcClient, "tm.event='NewRound'", "newRoundSubscriber")
    if err != nil {
        log.Fatal(err)
    }
	log.Printf("Bot process started... Listening on %s",b.rpcEndpoint)
    go b.processEvents(newBlockEventCh, newProposalEventCh)

    // Wait indefinitely
    select {}
}

func (b *Bot) Stop() {
    if b.rpcClient != nil {
        b.rpcClient.Stop()
    }
}

func (b *Bot) subscribeToEvent(rpcClient *chttp.HTTP, query, subscriber string) (<-chan ctypes.ResultEvent, error) {
    ctx := context.Background()
    eventCh, err := b.rpcClient.Subscribe(ctx, subscriber, query)
    if err != nil {
        return nil, err
    }
    return eventCh, nil
}

func (b *Bot) processEvents(newBlockEventCh, newProposalEventCh <-chan ctypes.ResultEvent) {
    validatorMissed := 0
    var validatorAddress string
    var missedThreshold int64
    var validatorDown bool
    var lastMissedMessageBlock int64
    var repeatThreshold int64
    for {
        select {
        case event := <-newBlockEventCh:
            blockEvent, ok := event.Data.(tmtypes.EventDataNewBlock)
            validatorAddress = b.cfg.GetValidatorAddress()
            missedThreshold = b.cfg.GetMissedThreshold()
            repeatThreshold = b.cfg.GetRepeatThreshold()
            if !ok {
                log.Printf("Unexpected event type for NewBlock: %T", event.Data)
                continue
            }

            validatorSigned := b.isValidatorSigned(blockEvent.Block.LastCommit.Signatures)
            if validatorSigned {
                if validatorDown {
                    resolvedEvent := config.ValidatorResolvedEvent{
                        ValidatorAddress: validatorAddress,
                    }
                    b.validatorResolvedCh <- resolvedEvent
                    validatorDown = false
                }
                validatorMissed = 0
                lastMissedMessageBlock = 0
            } else {
                validatorMissed++
                log.Printf("Validator %s did not sign the block %d\n", validatorAddress, blockEvent.Block.Height)

                if int64(validatorMissed) > missedThreshold {
                    if lastMissedMessageBlock == 0 || blockEvent.Block.Height-lastMissedMessageBlock >= repeatThreshold {
                        event := config.MissedBlocksEvent{
                            ValidatorAddress: validatorAddress,
                            MissedCount:      int64(validatorMissed),
                        }
                        b.missedBlocksCh <- event
                        lastMissedMessageBlock = blockEvent.Block.Height                    }
                }
            }

            if int64(validatorMissed) > 20 {
                if !validatorDown {
                    event := config.ValidatorDownEvent{
                        ValidatorAddress: validatorAddress,
                    }
                    b.validatorDownCh <- event
                    validatorDown = true
                }
            }
        }
    }
}

func (b *Bot) isValidatorSigned(signatures []tmtypes.CommitSig) bool {
    validatorAddress := b.cfg.GetValidatorAddress()
    for _, vote := range signatures {
        if vote.ValidatorAddress.String() == validatorAddress {
            return true
        }
    }
    return false
}


func (b *Bot) createRPCClient() (*chttp.HTTP, error) {
    rpcClient, err := chttp.New(b.rpcEndpoint, "/websocket")
    if err != nil {
        return nil, err
    }

    err = rpcClient.Start()
    if err != nil {
        return nil, err
    }

    return rpcClient, nil
}