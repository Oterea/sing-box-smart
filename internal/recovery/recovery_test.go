package recovery

import (
	"sing-box-smart/internal/domain"
	"testing"
	"time"
)

func sample(ms float64) domain.Sample { return domain.Sample{Success: true, DelayMS: &ms} }

func TestDetectsFailureToSuccess(t *testing.T) {
	s := Detect(nil, domain.Probe{At: time.Now(), Error: "timeout"}, domain.Probe{Success: true, DelayMS: 200}, .75, 100)
	if s.Kind != FailureToGood {
		t.Fatalf("kind=%q", s.Kind)
	}
}

func TestDetectsLargeLatencyDrop(t *testing.T) {
	h := []domain.Sample{sample(800), sample(820), sample(780)}
	s := Detect(h, domain.Probe{Success: true, DelayMS: 790}, domain.Probe{Success: true, DelayMS: 250}, .75, 100)
	if s.Kind != LatencyDrop || s.Baseline != 800 {
		t.Fatalf("signal=%+v", s)
	}
}

func TestIgnoresSmallLatencyChange(t *testing.T) {
	h := []domain.Sample{sample(800), sample(820), sample(780)}
	if s := Detect(h, domain.Probe{Success: true, DelayMS: 790}, domain.Probe{Success: true, DelayMS: 700}, .75, 100); s.Kind != None {
		t.Fatalf("signal=%+v", s)
	}
}
