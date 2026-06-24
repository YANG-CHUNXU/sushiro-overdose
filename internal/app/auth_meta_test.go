package app

import (
	"testing"
	"time"
)

// withTempAuthMeta 把 authMetaCache 换成一个隔离的内存副本，避免读写真实 ~/.sushiro。
// 注意：save 仍会落盘，但测试用 t.TempDir 改写 HOME 由调用方负责；这里只验算法。
func resetAuthMetaCacheForTest(st *authMetaState) func() {
	authMetaMu.Lock()
	prev := authMetaCache
	authMetaCache = st
	authMetaMu.Unlock()
	return func() {
		authMetaMu.Lock()
		authMetaCache = prev
		authMetaMu.Unlock()
	}
}

func TestMedianLifespanHours(t *testing.T) {
	cases := []struct {
		name string
		in   []float64
		want float64
	}{
		{"样本不足返回0", []float64{100}, 0},
		{"奇数取中位", []float64{24, 48, 72}, 48},
		{"偶数取均值", []float64{24, 48, 72, 96}, 60},
		{"乱序也对", []float64{96, 24, 72, 48}, 60},
		{"空", nil, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := medianLifespanHoursLocked(&authMetaState{LifespanHours: c.in})
			if got != c.want {
				t.Fatalf("median(%v)=%v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestAuthMetaSnapshotSoftWarn(t *testing.T) {
	// 历史寿命中位数 = 100h；年龄 90h ≥ 100*0.8=80h → soft_warn 应为 true。
	st := &authMetaState{
		CapturedAt:    time.Now().Add(-90 * time.Hour),
		CaptureMethod: captureMethodMobileProxy,
		LifespanHours: []float64{96, 104}, // median 100
	}
	restore := resetAuthMetaCacheForTest(st)
	defer restore()

	out := getAuthMeta()
	if !out.SoftWarn {
		t.Fatalf("年龄接近寿命应触发 soft_warn，got=%+v", out)
	}
	if out.CaptureMethodLabel != "手机抓包导入" {
		t.Fatalf("capture method label 错误: %q", out.CaptureMethodLabel)
	}
	if out.MedianLifespanDays == 0 {
		t.Fatalf("应给出中位寿命天数")
	}
}

func TestAuthMetaSnapshotFreshNoWarn(t *testing.T) {
	// 刚捕获、且寿命样本充足但年龄很小 → 不该 soft_warn。
	st := &authMetaState{
		CapturedAt:    time.Now().Add(-2 * time.Hour),
		LifespanHours: []float64{96, 104},
	}
	restore := resetAuthMetaCacheForTest(st)
	defer restore()

	if getAuthMeta().SoftWarn {
		t.Fatalf("新鲜凭证不该 soft_warn")
	}
}

func TestHumanizeDuration(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0.5, "不到 1 小时"},
		{5, "5 小时"},
		{36, "1.5 天"},
		{240, "10 天"},
	}
	for _, c := range cases {
		if got := humanizeDuration(c.in); got != c.want {
			t.Errorf("humanizeDuration(%v)=%q want %q", c.in, got, c.want)
		}
	}
}
