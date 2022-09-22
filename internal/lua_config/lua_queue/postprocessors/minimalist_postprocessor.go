package postprocessors

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Shopify/goqueuesim/internal/throttle/queue/impl/redis_queue"

	"github.com/rs/zerolog/log"
)

type MinimalistPostprocessor struct{
	MaxCps int64
}

func (p *MinimalistPostprocessor) PostprocessLuaResult(
	methodName string,
	params redis_queue.PostprocessParams,
) redis_queue.PostprocessResult {
	resultTuple := params.ResultTuple
	c := params.C
	switch methodName {
	case "add":
		binIdx := resultTuple[2].(int64)
		totalClients := resultTuple[3].(int64)

		remMillisecsToPoll := (float64(totalClients) / float64(p.MaxCps)) * 1000
		remDurToPoll := time.Duration(remMillisecsToPoll) * time.Millisecond

		c.SetThrottleCookieVal(redis_queue.LuaUserBinKey, strconv.FormatInt(binIdx, 10))
		c.SetThrottleCookieVal(redis_queue.LuaUserPosKey, strconv.FormatInt(totalClients, 10))
		c.AdvisePollAfter(time.Now().Add(remDurToPoll))

		log.Debug().Msg(fmt.Sprintf(resultTuple[1].(string)))
		return redis_queue.PostprocessResult{}
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

		return redis_queue.PostprocessResult{PollResult: isCandidateToProceed}
	case "size":
		log.Debug().Msg(fmt.Sprintf(resultTuple[1].(string)))
		totalClients := resultTuple[2].(int64)
		return redis_queue.PostprocessResult{SizeResult: totalClients}
	case "remove":
		fallthrough
	case "receive_feedback":
		fallthrough
	case "clear":
		fallthrough
	default:
		log.Debug().Msg(fmt.Sprintf(resultTuple[1].(string)))
		return redis_queue.PostprocessResult{}
	}
}
