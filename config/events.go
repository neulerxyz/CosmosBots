package config

type MissedBlocksEvent struct {
    ValidatorAddress string
    MissedCount      int64
}

type ValidatorDownEvent struct {
    ValidatorAddress string
}
