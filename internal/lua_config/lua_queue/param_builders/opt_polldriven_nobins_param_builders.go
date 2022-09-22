package param_builders

import (
	"strconv"
	"time"
)

type OptPollDrivenNobinsLuaKeysBuilder struct {
	BuilderGeneratedKeysSet map[string]bool
}
type OptPollDrivenNobinsLuaArgsBuilder struct {
	DefaultWorkingPosIncr string
	BuilderGeneratedArgsSet map[string]bool
}

func (b *OptPollDrivenNobinsLuaKeysBuilder) PreprocessKeys(methodName string, keys []string) []string {
	return keys
}

func (b *OptPollDrivenNobinsLuaArgsBuilder) PreprocessArgs(methodName string, args []string) []string {
	switch methodName {
	case "poll":
		// can imagine applying instead some dynamic logic to compute this incr
		workingBinPosIncr := b.DefaultWorkingPosIncr
		curUnixMillisecs := strconv.FormatInt(time.Now().Unix() * 1000, 10)
		return append(args, workingBinPosIncr, curUnixMillisecs)
	case "clear":
		fallthrough
	case "add":
		fallthrough
	case "remove":
		fallthrough
	case "receive_feedback":
		fallthrough
	default:
		return args
	}
}
