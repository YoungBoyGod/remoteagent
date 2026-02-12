package store

// Redis key 前缀定义，用于 Phase 2 任务调度系统
const (
	// 共享任务优先级队列（ZSET）
	RedisKeyQueueShared = "ra:queue:shared"
	// 独占任务优先级队列（ZSET）
	RedisKeyQueueExclusive = "ra:queue:exclusive"
	// 任务分布式锁前缀（String，SET NX EX）
	RedisKeyTaskLock = "ra:task:lock:"
	// Agent 容量快照前缀（Hash）
	RedisKeyAgentCapacity = "ra:agent:cap:"
	// Agent 在线状态集合（ZSET）
	RedisKeyAgentOnline = "ra:agent:online"
	// 任务缓存前缀（Hash/JSON）
	RedisKeyTaskCache = "ra:task:cache:"
)
