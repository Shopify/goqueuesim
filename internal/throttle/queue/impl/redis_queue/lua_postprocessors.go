package redis_queue

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Shopify/goqueuesim/internal/client"
	"github.com/Shopify/goqueuesim/internal/throttle/tracker"

	"github.com/rs/zerolog/log"
)

type PostprocessParams struct {
	ResultTuple []interface{}
	C           client.Client
	feedback    tracker.Feedback
}

type PostprocessResult struct {
	PollResult bool
	SizeResult int64
}

type LuaResultPostprocessor interface {
	PostprocessLuaResult(methodName string, params PostprocessParams) PostprocessResult
}

type DefaultLuaResultPostprocessor struct{}

func (p *DefaultLuaResultPostprocessor) PostprocessLuaResult(
	methodName string,
	params PostprocessParams,
) PostprocessResult {
	resultTuple := params.ResultTuple
	c := params.C
	switch methodName {
	case "add":
		binIdx := resultTuple[2].(int64)
		queuePos := resultTuple[3].(int64)
		remMillisecsToPoll := resultTuple[4].(int64)
		remDurToPoll := time.Duration(remMillisecsToPoll) * time.Millisecond

		c.SetThrottleCookieVal(LuaUserBinKey, strconv.FormatInt(binIdx, 10))
		c.SetThrottleCookieVal(LuaUserPosKey, strconv.FormatInt(queuePos, 10))
		c.AdvisePollAfter(time.Now().Add(remDurToPoll))

		log.Debug().Msg(fmt.Sprintf(resultTuple[1].(string)))
		return PostprocessResult{}
	case "poll":
		log.Debug().Msg(fmt.Sprintf(resultTuple[1].(string)))
		isCandidateToProceed := false
		pollResult := resultTuple[2].(string)
		switch pollResult {
		case "pass":
			isCandidateToProceed = true
		case "reject":
			isCandidateToProceed = false
		default:
			panic(fmt.Errorf("unexpected poll result: %s", pollResult))
		}

		return PostprocessResult{PollResult: isCandidateToProceed}
	case "size":
		log.Debug().Msg(fmt.Sprintf(resultTuple[1].(string)))
		totalClients := resultTuple[2].(int64)
		return PostprocessResult{SizeResult: totalClients}
	case "remove":
		fallthrough
	case "receive_feedback":
		fallthrough
	case "clear":
		fallthrough
	default:
		log.Debug().Msg(fmt.Sprintf(resultTuple[1].(string)))
		return PostprocessResult{}
	}
}
