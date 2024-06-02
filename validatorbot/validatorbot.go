package validatorbot

import (
	"context"
	"log"

	"github.com/neulerxyz/CosmosBots/config"
	tmtypes "github.com/cometbft/cometbft/types"
	chttp "github.com/cometbft/cometbft/rpc/client/http"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
)

type ValidatorState struct {
	MissedBlocks          int
	IsDown                bool
	LastMissedAlertHeight int64
	LastDownAlertHeight   int64
	LastSignedHeight      int64
	StartMissedHeight     int64
}

type ValidatorBot struct {
	cfg                 *config.Config
	rpcEndpoint         string
	rpcClient           *chttp.HTTP
	missedBlocksCh      chan config.MissedBlocksEvent
	validatorDownCh     chan config.ValidatorDownEvent
	validatorResolvedCh chan config.ValidatorResolvedEvent
	state               ValidatorState
}

func NewValidatorBot(cfg *config.Config,
	missedBlocksCh chan config.MissedBlocksEvent,
	validatorDownCh chan config.ValidatorDownEvent,
	validatorResolvedCh chan config.ValidatorResolvedEvent) *ValidatorBot {
	return &ValidatorBot{
		cfg:                 cfg,
		rpcEndpoint:         cfg.RPCEndpoint,
		missedBlocksCh:      missedBlocksCh,
		validatorDownCh:     validatorDownCh,
		validatorResolvedCh: validatorResolvedCh,
		state:               ValidatorState{},
	}
}

func (b *ValidatorBot) Start() error {
	var err error
	b.rpcClient, err = b.createRPCClient()
	if err != nil {
		log.Fatal(err)
	}

	newBlockEventCh, err := b.subscribeToEvent(b.rpcClient, "tm.event='NewBlock'", "newBlockSubscriber")
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("ValidatorBot process started... Listening on %s", b.rpcEndpoint)
    b.state.LastMissedAlertHeight = 0 //need to manually reset for the first run
	go b.processEvents(newBlockEventCh)

	// Wait indefinitely
	select {}
}

func (b *ValidatorBot) Stop() {
	if b.rpcClient != nil {
		b.rpcClient.Stop()
	}
}

func (b *ValidatorBot) subscribeToEvent(rpcClient *chttp.HTTP, query, subscriber string) (<-chan ctypes.ResultEvent, error) {
	ctx := context.Background()
	eventCh, err := b.rpcClient.Subscribe(ctx, subscriber, query)
	if err != nil {
		return nil, err
	}
	return eventCh, nil
}

func (b *ValidatorBot) processEvents(newBlockEventCh <-chan ctypes.ResultEvent) {
	for {
		select {
		case event := <-newBlockEventCh:
			blockEvent, ok := event.Data.(tmtypes.EventDataNewBlock)
			if !ok {
				log.Printf("Unexpected event type for NewBlock: %T", event.Data)
				continue
			}

			b.handleNewBlockEvent(blockEvent)
		}
	}
}

func (b *ValidatorBot) handleNewBlockEvent(blockEvent tmtypes.EventDataNewBlock) {
	validatorAddress := b.cfg.GetValidatorAddress()
	missedThreshold := b.cfg.GetMissedThreshold()
	repeatThreshold := b.cfg.GetRepeatThreshold()

	validatorSigned := b.isValidatorSigned(blockEvent.Block.LastCommit.Signatures)

	if validatorSigned {
		b.handleValidatorSigned(blockEvent.Block.Height)
	} else {
		b.handleValidatorMissedBlock(blockEvent.Block.Height, validatorAddress, missedThreshold, repeatThreshold)
	}
}

func (b *ValidatorBot) handleValidatorSigned(blockHeight int64) {
	if b.state.IsDown {
		resolvedEvent := config.ValidatorResolvedEvent{
			ValidatorAddress: b.cfg.GetValidatorAddress(),
			LastSignedHeight: blockHeight,
		}
		b.validatorResolvedCh <- resolvedEvent
		b.state.IsDown = false
		log.Printf("Validator %s is back online at block %d", b.cfg.GetValidatorAddress(), blockHeight)
	}
	b.state.MissedBlocks = 0
	b.state.LastMissedAlertHeight = 0
	b.state.LastDownAlertHeight = 0
	b.state.LastSignedHeight = blockHeight
	b.cfg.SetValidatorLastSignedHeight(blockHeight)
	b.cfg.SetValidatorIsDown(false)
	b.state.StartMissedHeight = 0
}

func (b *ValidatorBot) handleValidatorMissedBlock(blockHeight int64, validatorAddress string, missedThreshold, repeatThreshold int64) {
	if b.state.StartMissedHeight == 0 {
		b.state.StartMissedHeight = blockHeight
	}
	b.state.MissedBlocks++
	log.Printf("Validator %s did not sign the block %d\n", validatorAddress, blockHeight)

	if !b.state.IsDown && int64(b.state.MissedBlocks) > missedThreshold {
		if b.state.LastMissedAlertHeight == 0 || (blockHeight-b.state.LastMissedAlertHeight >= repeatThreshold) {
	        //log.Printf("Sending: %s did not sign the block %d\n", validatorAddress, blockHeight)
			b.sendMissedBlocksAlert(blockHeight, validatorAddress)
			b.state.LastMissedAlertHeight = blockHeight
		}

		if int64(b.state.MissedBlocks) > 20 {
			b.sendValidatorDownAlert(blockHeight, validatorAddress)
		}
	} else if b.state.IsDown && (blockHeight-b.state.LastDownAlertHeight >= repeatThreshold) {
		b.sendValidatorDownAlert(blockHeight, validatorAddress)
	}
}

func (b *ValidatorBot) sendMissedBlocksAlert(blockHeight int64, validatorAddress string) {
	event := config.MissedBlocksEvent{
		ValidatorAddress: validatorAddress,
		MissedCount:      int64(b.state.MissedBlocks),
		LastSignedHeight: b.state.StartMissedHeight,
	}
	b.missedBlocksCh <- event
}

func (b *ValidatorBot) sendValidatorDownAlert(blockHeight int64, validatorAddress string) {
	event := config.ValidatorDownEvent{
		ValidatorAddress: validatorAddress,
		MissedCount:      int64(b.state.MissedBlocks),
		LastSignedHeight: b.state.StartMissedHeight,
	}
	b.validatorDownCh <- event
	b.state.IsDown = true
	b.state.LastDownAlertHeight = blockHeight
	b.cfg.SetValidatorIsDown(true)
}

func (b *ValidatorBot) isValidatorSigned(signatures []tmtypes.CommitSig) bool {
	validatorAddress := b.cfg.GetValidatorAddress()
	for _, vote := range signatures {
		if vote.ValidatorAddress.String() == validatorAddress {
			return true
		}
	}
	return false
}

func (b *ValidatorBot) createRPCClient() (*chttp.HTTP, error) {
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
