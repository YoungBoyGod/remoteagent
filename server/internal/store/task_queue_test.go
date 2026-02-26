package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"luoyi2026/server/internal/store"
)

// newTestRedisStore 使用 miniredis 创建测试用 RedisStore
func newTestRedisStore(t *testing.T) (*store.RedisStore, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("启动 miniredis 失败: %v", err)
	}
	// 直接用 redis.Client 构造 RedisStore
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	rs := store.NewRedisStoreFromClient(client)
	return rs, mr
}

// --- EnqueueTask + DequeueTask 优先级排序 ---

func TestEnqueueAndDequeue_PriorityOrder(t *testing.T) {
	rs, mr := newTestRedisStore(t)
	defer mr.Close()
	ctx := context.Background()

	now := time.Now().UnixMilli()

	// 入队三个任务：优先级分别为 30、80、50
	// 优先级越高 score 越小，应排在前面
	if err := rs.EnqueueTask(ctx, "task-low", "shared", 30, now, ""); err != nil {
		t.Fatalf("EnqueueTask task-low: %v", err)
	}
	if err := rs.EnqueueTask(ctx, "task-high", "shared", 80, now+1, ""); err != nil {
		t.Fatalf("EnqueueTask task-high: %v", err)
	}
	if err := rs.EnqueueTask(ctx, "task-mid", "shared", 50, now+2, ""); err != nil {
		t.Fatalf("EnqueueTask task-mid: %v", err)
	}

	// 取出全部 3 个，应按优先级降序排列：high(80) > mid(50) > low(30)
	ids, err := rs.DequeueTask(ctx, "shared", 3)
	if err != nil {
		t.Fatalf("DequeueTask: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("期望 3 个任务，实际 %d", len(ids))
	}
	if ids[0] != "task-high" {
		t.Errorf("第 1 个应为 task-high，实际 %s", ids[0])
	}
	if ids[1] != "task-mid" {
		t.Errorf("第 2 个应为 task-mid，实际 %s", ids[1])
	}
	if ids[2] != "task-low" {
		t.Errorf("第 3 个应为 task-low，实际 %s", ids[2])
	}
}

// --- 同优先级按创建时间升序 ---

func TestEnqueueAndDequeue_SamePriority_FIFO(t *testing.T) {
	rs, mr := newTestRedisStore(t)
	defer mr.Close()
	ctx := context.Background()

	// 同优先级 50，按创建时间排序
	if err := rs.EnqueueTask(ctx, "task-a", "shared", 50, 1000, ""); err != nil {
		t.Fatalf("EnqueueTask task-a: %v", err)
	}
	if err := rs.EnqueueTask(ctx, "task-b", "shared", 50, 2000, ""); err != nil {
		t.Fatalf("EnqueueTask task-b: %v", err)
	}
	if err := rs.EnqueueTask(ctx, "task-c", "shared", 50, 1500, ""); err != nil {
		t.Fatalf("EnqueueTask task-c: %v", err)
	}

	ids, err := rs.DequeueTask(ctx, "shared", 3)
	if err != nil {
		t.Fatalf("DequeueTask: %v", err)
	}
	// 同优先级按 createdAtMs 升序：a(1000) < c(1500) < b(2000)
	if ids[0] != "task-a" || ids[1] != "task-c" || ids[2] != "task-b" {
		t.Errorf("同优先级排序错误，期望 [task-a, task-c, task-b]，实际 %v", ids)
	}
}

// --- exclusive 队列隔离 ---

func TestEnqueue_ExecModeIsolation(t *testing.T) {
	rs, mr := newTestRedisStore(t)
	defer mr.Close()
	ctx := context.Background()

	now := time.Now().UnixMilli()
	rs.EnqueueTask(ctx, "shared-1", "shared", 50, now, "")
	rs.EnqueueTask(ctx, "excl-1", "exclusive", 50, now, "")

	sharedIDs, _ := rs.DequeueTask(ctx, "shared", 10)
	exclIDs, _ := rs.DequeueTask(ctx, "exclusive", 10)

	if len(sharedIDs) != 1 || sharedIDs[0] != "shared-1" {
		t.Errorf("shared 队列应只有 shared-1，实际 %v", sharedIDs)
	}
	if len(exclIDs) != 1 || exclIDs[0] != "excl-1" {
		t.Errorf("exclusive 队列应只有 excl-1，实际 %v", exclIDs)
	}
}

// --- RemoveTask ---

func TestRemoveTask(t *testing.T) {
	rs, mr := newTestRedisStore(t)
	defer mr.Close()
	ctx := context.Background()

	rs.EnqueueTask(ctx, "task-rm", "shared", 50, 1000, "")
	rs.EnqueueTask(ctx, "task-keep", "shared", 50, 2000, "")

	if err := rs.RemoveTask(ctx, "task-rm", "shared", ""); err != nil {
		t.Fatalf("RemoveTask: %v", err)
	}

	ids, _ := rs.DequeueTask(ctx, "shared", 10)
	if len(ids) != 1 || ids[0] != "task-keep" {
		t.Errorf("移除后应只剩 task-keep，实际 %v", ids)
	}
}

// --- AcquireTaskLock 互斥性 ---

func TestAcquireTaskLock_Mutual_Exclusion(t *testing.T) {
	rs, mr := newTestRedisStore(t)
	defer mr.Close()
	ctx := context.Background()

	// agent-1 先获取锁
	ok, err := rs.AcquireTaskLock(ctx, "task-1", "agent-1", 30*time.Second)
	if err != nil {
		t.Fatalf("AcquireTaskLock agent-1: %v", err)
	}
	if !ok {
		t.Fatal("agent-1 应成功获取锁")
	}

	// agent-2 尝试获取同一任务的锁，应失败
	ok2, err := rs.AcquireTaskLock(ctx, "task-1", "agent-2", 30*time.Second)
	if err != nil {
		t.Fatalf("AcquireTaskLock agent-2: %v", err)
	}
	if ok2 {
		t.Fatal("agent-2 不应获取到已被 agent-1 持有的锁")
	}

	// agent-1 获取另一个任务的锁，应成功（不同任务互不影响）
	ok3, err := rs.AcquireTaskLock(ctx, "task-2", "agent-1", 30*time.Second)
	if err != nil {
		t.Fatalf("AcquireTaskLock task-2: %v", err)
	}
	if !ok3 {
		t.Fatal("agent-1 应能获取不同任务的锁")
	}
}

// --- AcquireTaskLock TTL 过期后可重新获取 ---

func TestAcquireTaskLock_Expiry(t *testing.T) {
	rs, mr := newTestRedisStore(t)
	defer mr.Close()
	ctx := context.Background()

	ok, _ := rs.AcquireTaskLock(ctx, "task-exp", "agent-1", 30*time.Second)
	if !ok {
		t.Fatal("agent-1 应成功获取锁")
	}

	// 快进时间使锁过期
	mr.FastForward(31 * time.Second)

	// 锁过期后 agent-2 应能获取
	ok2, err := rs.AcquireTaskLock(ctx, "task-exp", "agent-2", 30*time.Second)
	if err != nil {
		t.Fatalf("AcquireTaskLock after expiry: %v", err)
	}
	if !ok2 {
		t.Fatal("锁过期后 agent-2 应能获取")
	}
}

// --- ReleaseTaskLock 仅持有者可释放 ---

func TestReleaseTaskLock_OnlyOwner(t *testing.T) {
	rs, mr := newTestRedisStore(t)
	defer mr.Close()
	ctx := context.Background()

	rs.AcquireTaskLock(ctx, "task-rel", "agent-1", 30*time.Second)

	// agent-2 尝试释放 agent-1 的锁，应失败
	released, err := rs.ReleaseTaskLock(ctx, "task-rel", "agent-2")
	if err != nil {
		t.Fatalf("ReleaseTaskLock agent-2: %v", err)
	}
	if released {
		t.Fatal("非持有者 agent-2 不应能释放锁")
	}

	// agent-1 释放自己的锁，应成功
	released, err = rs.ReleaseTaskLock(ctx, "task-rel", "agent-1")
	if err != nil {
		t.Fatalf("ReleaseTaskLock agent-1: %v", err)
	}
	if !released {
		t.Fatal("持有者 agent-1 应能释放锁")
	}

	// 释放后其他 agent 可以获取
	ok, _ := rs.AcquireTaskLock(ctx, "task-rel", "agent-2", 30*time.Second)
	if !ok {
		t.Fatal("锁释放后 agent-2 应能获取")
	}
}

// --- ReleaseTaskLock 对不存在的锁 ---

func TestReleaseTaskLock_NonExistent(t *testing.T) {
	rs, mr := newTestRedisStore(t)
	defer mr.Close()
	ctx := context.Background()

	released, err := rs.ReleaseTaskLock(ctx, "no-such-task", "agent-1")
	if err != nil {
		t.Fatalf("ReleaseTaskLock non-existent: %v", err)
	}
	if released {
		t.Fatal("不存在的锁不应返回 released=true")
	}
}

// --- UpdatePriority 重排序 ---

func TestUpdatePriority_Reorder(t *testing.T) {
	rs, mr := newTestRedisStore(t)
	defer mr.Close()
	ctx := context.Background()

	now := time.Now().UnixMilli()

	// 初始：task-a 优先级 80，task-b 优先级 30
	rs.EnqueueTask(ctx, "task-a", "shared", 80, now, "")
	rs.EnqueueTask(ctx, "task-b", "shared", 30, now+1, "")

	// 验证初始顺序：task-a(80) 在前
	ids, _ := rs.DequeueTask(ctx, "shared", 2)
	if ids[0] != "task-a" {
		t.Fatalf("初始排序错误，期望 task-a 在前，实际 %v", ids)
	}

	// 将 task-b 优先级提升到 90
	if err := rs.UpdatePriority(ctx, "task-b", "shared", 90, now+1); err != nil {
		t.Fatalf("UpdatePriority: %v", err)
	}

	// 重排后 task-b(90) 应在 task-a(80) 前面
	ids, _ = rs.DequeueTask(ctx, "shared", 2)
	if ids[0] != "task-b" {
		t.Errorf("重排后 task-b 应在前，实际 %v", ids)
	}
	if ids[1] != "task-a" {
		t.Errorf("重排后 task-a 应在后，实际 %v", ids)
	}
}

// --- SetAgentCapacity + GetAgentCapacity ---

func TestAgentCapacity_SetAndGet(t *testing.T) {
	rs, mr := newTestRedisStore(t)
	defer mr.Close()
	ctx := context.Background()

	err := rs.SetAgentCapacity(ctx, "agent-cap-1", 4, 2, false)
	if err != nil {
		t.Fatalf("SetAgentCapacity: %v", err)
	}

	cap, err := rs.GetAgentCapacity(ctx, "agent-cap-1")
	if err != nil {
		t.Fatalf("GetAgentCapacity: %v", err)
	}
	if cap == nil {
		t.Fatal("GetAgentCapacity 返回 nil")
	}
	if cap.MaxConcurrent != 4 {
		t.Errorf("MaxConcurrent 期望 4，实际 %d", cap.MaxConcurrent)
	}
	if cap.RunningShared != 2 {
		t.Errorf("RunningShared 期望 2，实际 %d", cap.RunningShared)
	}
	if cap.RunningExclusive {
		t.Error("RunningExclusive 期望 false")
	}
}

// --- GetAgentCapacity 不存在时返回 nil ---

func TestAgentCapacity_NotFound(t *testing.T) {
	rs, mr := newTestRedisStore(t)
	defer mr.Close()
	ctx := context.Background()

	cap, err := rs.GetAgentCapacity(ctx, "no-such-agent")
	if err != nil {
		t.Fatalf("GetAgentCapacity: %v", err)
	}
	if cap != nil {
		t.Fatal("不存在的 agent 应返回 nil")
	}
}

// --- AgentCapacity TTL 过期 ---

func TestAgentCapacity_TTLExpiry(t *testing.T) {
	rs, mr := newTestRedisStore(t)
	defer mr.Close()
	ctx := context.Background()

	rs.SetAgentCapacity(ctx, "agent-ttl", 2, 1, true)

	// 快进 91 秒，超过 90s TTL
	mr.FastForward(91 * time.Second)

	cap, err := rs.GetAgentCapacity(ctx, "agent-ttl")
	if err != nil {
		t.Fatalf("GetAgentCapacity after TTL: %v", err)
	}
	if cap != nil {
		t.Fatal("TTL 过期后应返回 nil")
	}
}

// --- QueueLen ---

func TestQueueLen(t *testing.T) {
	rs, mr := newTestRedisStore(t)
	defer mr.Close()
	ctx := context.Background()

	// 空队列
	n, err := rs.QueueLen(ctx, "shared")
	if err != nil {
		t.Fatalf("QueueLen: %v", err)
	}
	if n != 0 {
		t.Errorf("空队列长度应为 0，实际 %d", n)
	}

	// 入队 2 个
	rs.EnqueueTask(ctx, "t1", "shared", 50, 1000, "")
	rs.EnqueueTask(ctx, "t2", "shared", 50, 2000, "")

	n, _ = rs.QueueLen(ctx, "shared")
	if n != 2 {
		t.Errorf("队列长度应为 2，实际 %d", n)
	}

	// 移除 1 个
	rs.RemoveTask(ctx, "t1", "shared", "")
	n, _ = rs.QueueLen(ctx, "shared")
	if n != 1 {
		t.Errorf("移除后队列长度应为 1，实际 %d", n)
	}
}

// --- CalculateScore 验证 ---

func TestCalculateScore(t *testing.T) {
	// priority=100 时 score 最小（排最前）
	s100 := store.CalculateScore(100, 0)
	s50 := store.CalculateScore(50, 0)
	s1 := store.CalculateScore(1, 0)

	if s100 >= s50 {
		t.Errorf("priority=100 的 score(%f) 应小于 priority=50 的 score(%f)", s100, s50)
	}
	if s50 >= s1 {
		t.Errorf("priority=50 的 score(%f) 应小于 priority=1 的 score(%f)", s50, s1)
	}

	// 同优先级，createdAtMs 越小 score 越小
	sEarly := store.CalculateScore(50, 1000)
	sLate := store.CalculateScore(50, 2000)
	if sEarly >= sLate {
		t.Errorf("早创建的 score(%f) 应小于晚创建的 score(%f)", sEarly, sLate)
	}
}
