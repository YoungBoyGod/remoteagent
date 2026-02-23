package observability

import (
	"testing"
)

// TestNewMetrics 测试 NewMetrics 构造函数：返回非 nil 实例，histogram 和 startTime 已初始化
func TestNewMetrics(t *testing.T) {
	m := NewMetrics()
	if m == nil {
		t.Fatal("NewMetrics() returned nil")
	}
	// 验证 histogram 已初始化
	if m.httpLatencyMs == nil {
		t.Fatal("httpLatencyMs histogram not initialized")
	}
	// 验证启动时间已设置
	if m.startTime.IsZero() {
		t.Fatal("startTime should be set")
	}
}

// TestCounter 测试 counter 原子计数器：初始值为 0，累加后值正确
func TestCounter(t *testing.T) {
	var c counter
	// 初始值应为 0
	if c.Get() != 0 {
		t.Errorf("initial counter = %d, want 0", c.Get())
	}
	c.Add(1)
	c.Add(3)
	// 累加后应为 4
	if c.Get() != 4 {
		t.Errorf("counter = %d, want 4", c.Get())
	}
}

// TestGauge 测试 gauge 原子仪表：支持 Set 和 Add（含负数）操作
func TestGauge(t *testing.T) {
	var g gauge
	// 初始值应为 0
	if g.Get() != 0 {
		t.Errorf("initial gauge = %d, want 0", g.Get())
	}
	g.Set(10)
	if g.Get() != 10 {
		t.Errorf("gauge = %d, want 10", g.Get())
	}
	// 支持负数增量
	g.Add(-3)
	if g.Get() != 7 {
		t.Errorf("gauge = %d, want 7", g.Get())
	}
}

// TestHistogram 测试 histogram 直方图：观测值正确分桶，sum 和 count 累计正确
func TestHistogram(t *testing.T) {
	h := newHistogram([]float64{10, 50, 100})
	h.Observe(5)
	h.Observe(25)
	h.Observe(75)
	h.Observe(200)

	buckets, counts, sum, count := h.Snapshot()
	// 总观测次数应为 4
	if count != 4 {
		t.Errorf("count = %d, want 4", count)
	}
	// 总和应为所有观测值之和
	wantSum := 5.0 + 25.0 + 75.0 + 200.0
	if sum != wantSum {
		t.Errorf("sum = %f, want %f", sum, wantSum)
	}
	if len(buckets) != 3 {
		t.Fatalf("buckets len = %d, want 3", len(buckets))
	}
	// 桶 10: 只有值 5 落入
	if counts[0] != 1 {
		t.Errorf("bucket[10] count = %d, want 1", counts[0])
	}
	// 桶 50: 值 5 和 25 落入
	if counts[1] != 2 {
		t.Errorf("bucket[50] count = %d, want 2", counts[1])
	}
	// 桶 100: 值 5、25、75 落入
	if counts[2] != 3 {
		t.Errorf("bucket[100] count = %d, want 3", counts[2])
	}
}

// TestHistogramSortsBuckets 测试 histogram 构造时自动对桶边界排序
func TestHistogramSortsBuckets(t *testing.T) {
	h := newHistogram([]float64{100, 10, 50})
	// 无论输入顺序如何，桶边界应升序排列
	if h.buckets[0] != 10 || h.buckets[1] != 50 || h.buckets[2] != 100 {
		t.Errorf("buckets not sorted: %v", h.buckets)
	}
}
