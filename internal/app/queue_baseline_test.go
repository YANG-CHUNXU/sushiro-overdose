package app

import "testing"

func TestQueueLiveStoreOnlineOpen(t *testing.T) {
	cases := []struct {
		status string
		want   bool
	}{
		{"ONLINE", true},
		{"online", true},
		{"ON", true},
		{"OFFLINE_CLOSED", false},
		{"OFFLINE", false},
		{"CLOSED", false},
		{"", false},
		{"  ONLINE  ", true},
	}
	for _, c := range cases {
		if got := queueLiveStoreOnlineOpen(QueueLiveStore{NetTicketStatus: c.status}); got != c.want {
			t.Errorf("queueLiveStoreOnlineOpen(%q) = %v, want %v", c.status, got, c.want)
		}
	}
}

func TestNormalizeQueueBaselineConfig(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{0, queueBaselineDefaultMinutes},
		{-5, queueBaselineDefaultMinutes},
		{3, 3},
		{5000, 1440},
	}
	for _, c := range cases {
		got := NormalizeQueueBaselineConfig(QueueBaselineConfig{IntervalMinutes: c.in})
		if got.IntervalMinutes != c.want {
			t.Errorf("NormalizeQueueBaselineConfig(%d) = %d, want %d", c.in, got.IntervalMinutes, c.want)
		}
	}
}
