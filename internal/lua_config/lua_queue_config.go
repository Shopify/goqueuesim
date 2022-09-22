package lua_config

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/Shopify/goqueuesim/internal/lua_config/lua_queue"

	"github.com/go-redis/redis/v7"
)

func loadFileStr(filepath string) string {
	buffer, err := ioutil.ReadFile(filepath)
	if err != nil {
		panic(fmt.Errorf("failed opening file at path '%s'", filepath))
	}
	return string(buffer)
}

func SetDefaultLuaQueueParams() (*lua_queue.LuaQueueParams) {
	luaMethodToShaMap := map[string]string{}
	return lua_queue.MakeNoopQueueParams(luaMethodToShaMap)
}

func ConfigureRedisLua(
	dirpath string,
	redisClient *redis.Client,
	constants lua_queue.LuaQueueConstants,
) *lua_queue.LuaQueueParams {
	if constants.QueueType != "lua_driven_bins_queue" {
		panic(fmt.Errorf("can't configure Redis lua for non-redis queue"))
	}
	luaMethodToShaMap := map[string]string{}
	files, err := ioutil.ReadDir(dirpath)
	if err != nil {
		panic(fmt.Errorf("failed opening lua queue directory at path '%s'", dirpath))
	}
	for _, f := range files {
		fileName := f.Name()
		ext := filepath.Ext(fileName)
		if ext != ".lua" {
			continue
		}
		action := strings.TrimSuffix(fileName, ext)
		luaPath := filepath.Join(dirpath, fileName)
		luaScriptStr := loadFileStr(luaPath)
		var actionSha string
		if actionSha, err = redisClient.ScriptLoad(luaScriptStr).Result(); err != nil {
			panic(err)
		}
		luaMethodToShaMap[action] = actionSha
	}
	luaStrSlice := strings.Split(dirpath, "/")
	luaDirName := luaStrSlice[len(luaStrSlice)-1]
	queueParams, err := prepareLuaQueueParams(luaDirName, luaMethodToShaMap, constants)
	if err != nil {
		panic(err)
	}
	return queueParams
}

func prepareLuaQueueParams(
	luaDirName string,
	luaMethodToShaMap map[string]string,
	constants lua_queue.LuaQueueConstants,
) (*lua_queue.LuaQueueParams, error) {
	if constants.QueueType != "lua_driven_bins_queue" {
		return nil, fmt.Errorf("can't build params for non-lua queue type")
	}
	switch luaDirName {
	case "noop":
		return lua_queue.MakeNoopQueueParams(luaMethodToShaMap), nil
	case "polling_feedback_driven":
		return lua_queue.MakePollingFeedbackBinsQueueParams(constants, luaMethodToShaMap), nil
	case "fairness_interval_driven":
		return lua_queue.MakeFairnessIntervalDrivenQueueParams(constants, luaMethodToShaMap), nil
	case "opt_polldriven_notimers":
		return lua_queue.MakeOptNotimersPollDrivenParams(constants, luaMethodToShaMap), nil
	case "opt_polldriven_ktimers":
		return lua_queue.MakeOptKTimersPollDrivenParams(constants, luaMethodToShaMap), nil
	case "opt_fairness_interval":
		return lua_queue.MakeOptFairnessIntervalParams(constants, luaMethodToShaMap), nil
	case "opt_polldriven_nobins":
		return lua_queue.MakeOptPollDrivenNobinsParams(constants, luaMethodToShaMap), nil
	default:
		panic(fmt.Errorf("unknown lua input directory: %s", luaDirName))
	}
}
