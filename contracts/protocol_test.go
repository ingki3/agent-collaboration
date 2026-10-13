package contracts

import (
	"testing"
	"time"
)

func TestStallTimeoutFor(t *testing.T) {
	cases := []struct {
		kind RuntimeKind
		want time.Duration
	}{
		{RuntimeClaudeCode, StallTimeout},
		{RuntimeHermes, StallTimeoutHermes},
		{"", StallTimeout},
		{RuntimeKind("future_kind"), StallTimeout},
	}
	for _, c := range cases {
		if got := StallTimeoutFor(c.kind); got != c.want {
			t.Errorf("StallTimeoutFor(%q) = %v, want %v", c.kind, got, c.want)
		}
	}
	if StallTimeoutHermes <= StallTimeout {
		t.Errorf("StallTimeoutHermes (%v) must be longer than StallTimeout (%v) — hermes has no reliable activity signal", StallTimeoutHermes, StallTimeout)
	}
}
