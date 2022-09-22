package redis_queue

type KeysPreprocessor interface {
	PreprocessKeys(methodName string, keys []string) []string
}

type ArgsPreprocessor interface {
	PreprocessArgs(methodName string, args []string) []string
}

type NoopLuaPreprocessor struct{}

func (r *NoopLuaPreprocessor) PreprocessKeys(methodName string, keys []string) []string {
	return keys
}

func (r *NoopLuaPreprocessor) PreprocessArgs(methodName string, args []string) []string {
	return args
}
