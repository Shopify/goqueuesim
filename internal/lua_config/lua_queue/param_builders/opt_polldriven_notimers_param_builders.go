package param_builders

import (
	"strconv"
	"time"
)

type OptPollDrivenNotimersLuaKeysBuilder struct {
	BuilderGeneratedKeysSet map[string]bool
}
type OptPollDrivenNotimersLuaArgsBuilder struct {
	BuilderGeneratedArgsSet map[string]bool
}

func (b *OptPollDrivenNotimersLuaKeysBuilder) PreprocessKeys(methodName string, keys []string) []string {
	return keys
}

func (b *OptPollDrivenNotimersLuaArgsBuilder) PreprocessArgs(methodName string, args []string) []string {
	switch methodName {
	case "poll":
		curUnixMillisecs := strconv.FormatInt(time.Now().Unix() * 1000, 10)
		result := append(args, curUnixMillisecs)
		return result
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
