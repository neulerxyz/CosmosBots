package config

type MissedBlocksEvent struct {
    ValidatorAddress string
    MissedCount      int64
    LastSignedHeight int64
}

type ValidatorDownEvent struct {
    ValidatorAddress string
    LastSignedHeight  int64
}

type ValidatorResolvedEvent struct {
    ValidatorAddress string
    LastSignedHeight  int64
}

