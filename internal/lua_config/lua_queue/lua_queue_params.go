package lua_queue

import "github.com/Shopify/goqueuesim/internal/throttle/queue/impl/redis_queue"

type LuaQueueParams struct {
	ShopScopePrefix                       string
	LuaMethodToShopScopePrefixedKeys      map[string][]string
	LuaMethodToClientScopeNonPrefixedKeys map[string][]string
	LuaMethodToConstantArgsMap            map[string][]string
	LuaMethodToShaMap                     map[string]string
	KeysBuilder                           redis_queue.KeysPreprocessor
	ArgsBuilder                           redis_queue.ArgsPreprocessor
	Postprocessor                         redis_queue.LuaResultPostprocessor
}
