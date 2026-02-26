package store

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore 封装 Redis 任务队列操作
type RedisStore struct {
	client *redis.Client
}

// NewRedisStore 初始化 Redis 客户端
func NewRedisStore(addr, password string, db int) (*RedisStore, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     20,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return &RedisStore{client: client}, nil
}

// NewRedisStoreFromClient 从已有的 redis.Client 创建 RedisStore（用于测试注入）
func NewRedisStoreFromClient(client *redis.Client) *RedisStore {
	return &RedisStore{client: client}
}

// Close 关闭 Redis 连接
func (r *RedisStore) Close() error {
	return r.client.Close()
}

// Client 返回底层 redis.Client，供需要直接操作的场景使用
func (r *RedisStore) Client() *redis.Client {
	return r.client
}

// CalculateScore 计算 ZSET score
// score = (100 - priority) * 10^13 + created_at_ms
// priority 越高 score 越小，排在前面；同优先级按创建时间升序
func CalculateScore(priority int, createdAtMs int64) float64 {
	return float64(100-priority)*1e13 + float64(createdAtMs)
}

// EnqueueTask 将任务入队到对应的优先级队列
// targetAgentID 非空时入 agent 专属队列，否则入全局队列
func (r *RedisStore) EnqueueTask(ctx context.Context, taskID, execMode string, priority int, createdAtMs int64, targetAgentID string) error {
	key := agentQueueKey(targetAgentID, execMode)
	score := CalculateScore(priority, createdAtMs)
	return r.client.ZAdd(ctx, key, redis.Z{
		Score:  score,
		Member: taskID,
	}).Err()
}

// DequeueTask 从队列中取出 count 个最高优先级的候选任务（不移除）
func (r *RedisStore) DequeueTask(ctx context.Context, execMode string, count int64) ([]string, error) {
	key := queueKey(execMode)
	return r.client.ZRange(ctx, key, 0, count-1).Result()
}

// RemoveTask 从队列中移除任务
// targetAgentID 非空时从 agent 专属队列移除，否则从全局队列移除
func (r *RedisStore) RemoveTask(ctx context.Context, taskID, execMode, targetAgentID string) error {
	key := agentQueueKey(targetAgentID, execMode)
	return r.client.ZRem(ctx, key, taskID).Err()
}

// DequeueAgentTask 从 agent 专属队列取出候选任务（不移除）
func (r *RedisStore) DequeueAgentTask(ctx context.Context, agentID, execMode string, count int64) ([]string, error) {
	key := RedisKeyQueueAgentPrefix + agentID + ":" + execMode
	return r.client.ZRange(ctx, key, 0, count-1).Result()
}

// UpdatePriority 动态调整任务在队列中的优先级
func (r *RedisStore) UpdatePriority(ctx context.Context, taskID, execMode string, newPriority int, createdAtMs int64) error {
	key := queueKey(execMode)
	score := CalculateScore(newPriority, createdAtMs)
	// ZAddXX: 只更新已存在的成员
	result := r.client.ZAddXX(ctx, key, redis.Z{
		Score:  score,
		Member: taskID,
	})
	if result.Err() != nil {
		return result.Err()
	}
	if result.Val() == 0 {
		// ZAddXX 返回 0 表示没有更新（成员不存在），检查是否确实不在队列中
		exists, err := r.client.ZScore(ctx, key, taskID).Result()
		if err == redis.Nil {
			return fmt.Errorf("task %s not found in queue %s", taskID, key)
		}
		if err != nil {
			return err
		}
		// 成员存在但 score 没变（ZAddXX 返回 0 也可能是 score 相同）
		_ = exists
	}
	return nil
}

// AcquireTaskLock 获取任务认领分布式锁
// 使用 SET NX EX 实现，返回是否成功获取
func (r *RedisStore) AcquireTaskLock(ctx context.Context, taskID, agentID string, ttl time.Duration) (bool, error) {
	key := RedisKeyTaskLock + taskID
	return r.client.SetNX(ctx, key, agentID, ttl).Result()
}

// ReleaseTaskLock 释放任务认领锁
// 使用 Lua 脚本确保只有持有者才能释放
func (r *RedisStore) ReleaseTaskLock(ctx context.Context, taskID, agentID string) (bool, error) {
	key := RedisKeyTaskLock + taskID
	script := redis.NewScript(`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		end
		return 0
	`)
	result, err := script.Run(ctx, r.client, []string{key}, agentID).Int64()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

// SetAgentCapacity 缓存 Agent 容量快照
func (r *RedisStore) SetAgentCapacity(ctx context.Context, agentID string, maxConcurrent, runningShared int, runningExclusive bool) error {
	key := RedisKeyAgentCapacity + agentID
	exclVal := "0"
	if runningExclusive {
		exclVal = "1"
	}
	pipe := r.client.Pipeline()
	pipe.HSet(ctx, key, map[string]interface{}{
		"max_concurrent":    strconv.Itoa(maxConcurrent),
		"running_shared":    strconv.Itoa(runningShared),
		"running_exclusive": exclVal,
	})
	pipe.Expire(ctx, key, 90*time.Second)
	_, err := pipe.Exec(ctx)
	return err
}

// AgentCapacity Agent 容量快照
type AgentCapacity struct {
	MaxConcurrent    int
	RunningShared    int
	RunningExclusive bool
}

// GetAgentCapacity 读取 Agent 容量快照
func (r *RedisStore) GetAgentCapacity(ctx context.Context, agentID string) (*AgentCapacity, error) {
	key := RedisKeyAgentCapacity + agentID
	result, err := r.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, nil
	}

	maxConcurrent, _ := strconv.Atoi(result["max_concurrent"])
	runningShared, _ := strconv.Atoi(result["running_shared"])
	runningExclusive := result["running_exclusive"] == "1"

	return &AgentCapacity{
		MaxConcurrent:    maxConcurrent,
		RunningShared:    runningShared,
		RunningExclusive: runningExclusive,
	}, nil
}

// QueueLen 返回指定队列的长度
func (r *RedisStore) QueueLen(ctx context.Context, execMode string) (int64, error) {
	key := queueKey(execMode)
	return r.client.ZCard(ctx, key).Result()
}

// queueKey 根据 exec_mode 返回对应的 Redis key
func queueKey(execMode string) string {
	if execMode == "exclusive" {
		return RedisKeyQueueExclusive
	}
	return RedisKeyQueueShared
}

// agentQueueKey 根据 targetAgentID 和 execMode 返回对应的 Redis key
// targetAgentID 非空时返回 agent 专属队列 key，否则返回全局队列 key
func agentQueueKey(targetAgentID, execMode string) string {
	if targetAgentID != "" {
		return RedisKeyQueueAgentPrefix + targetAgentID + ":" + execMode
	}
	return queueKey(execMode)
}
