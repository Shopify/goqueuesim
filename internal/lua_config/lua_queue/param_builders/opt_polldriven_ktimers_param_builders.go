package param_builders

import (
	"strconv"
	"time"
)

type OptPollDrivenKTimersLuaKeysBuilder struct {
	BuilderGeneratedKeysSet map[string]bool
}
type OptPollDrivenKTimersLuaArgsBuilder struct {
	BuilderGeneratedArgsSet map[string]bool
}

func (b *OptPollDrivenKTimersLuaKeysBuilder) PreprocessKeys(methodName string, keys []string) []string {
	return keys
}

func (b *OptPollDrivenKTimersLuaArgsBuilder) PreprocessArgs(methodName string, args []string) []string {
	switch methodName {
	case "poll":
		curUnixSeconds := time.Now().Unix()
		prevSecondKey := strconv.FormatInt(curUnixSeconds - 1, 10)
		curSecondKey := strconv.FormatInt(curUnixSeconds, 10)
		return append(args, prevSecondKey, curSecondKey)
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
