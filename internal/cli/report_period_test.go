package cli

import (
	"testing"
	"time"
)

func TestReportFigure_ToTheSecondThenTheMinute(t *testing.T) {
	// An average needn't be whole seconds. htt gui's API rounds it to
	// the second; the report does too, before it drops the seconds, so
	// both show the same minute.
	cases := []struct {
		d    time.Duration
		want string
	}{
		{59*time.Minute + 59*time.Second + 600*time.Millisecond, "1h 0m"},
		{59*time.Minute + 59*time.Second + 400*time.Millisecond, "59m"},
		{7*time.Hour + 56*time.Minute + 15*time.Second, "7h 56m"},
		{29 * time.Second, "0m"},
		{0, "0m"},
	}
	for _, c := range cases {
		if got := reportFigure(c.d); got != c.want {
			t.Errorf("reportFigure(%v) = %q, want %q", c.d, got, c.want)
		}
	}
	if got := reportFigureIf(time.Hour, false); got != "-" {
		t.Errorf("reportFigureIf(1h, false) = %q, want -", got)
	}
}

func TestReportPercent_Rounded(t *testing.T) {
	cases := []struct {
		x    float64
		want string
	}{
		{0.1111, "11%"}, {0.815, "82%"}, {1, "100%"}, {-0.464, "-46%"}, {4.625, "463%"}, {-0.004, "0%"}, {0, "0%"},
	}
	for _, c := range cases {
		if got := reportPercent(c.x); got != c.want {
			t.Errorf("reportPercent(%v) = %q, want %q", c.x, got, c.want)
		}
	}
}
