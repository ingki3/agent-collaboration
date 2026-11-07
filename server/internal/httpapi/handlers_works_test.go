package httpapi

import "testing"

// TestShortTitleOf — openWork's default title (실사용 요청 2026-11-07): a goal
// with no line break must still get a skimmable headline, not a verbatim
// copy of the whole goal (the WorkPanel bug this fixes: title == goal hid
// the title row entirely).
func TestShortTitleOf(t *testing.T) {
	cases := []struct {
		name string
		goal string
		want string
	}{
		{"short goal unchanged", "릴리스 노트 정리", "릴리스 노트 정리"},
		{"first line only, rest dropped", "릴리스 노트 정리\n세부는 나중에", "릴리스 노트 정리"},
		{"exactly at the limit unchanged", repeatRune('가', shortTitleMaxRunes), repeatRune('가', shortTitleMaxRunes)},
		{
			"long single-line goal clipped at a word boundary",
			"사용자들이 자신의 에이전트를 보내서 에이전트들끼리 자동으로 게임을 하는 플랫폼 예를 들면 바둑 같은 전략 게임을 들 수 있다",
			"사용자들이 자신의 에이전트를 보내서 에이전트들끼리 자동으로 게임을 하는 플랫폼 예를 들면…",
		},
		{"no word boundary before the limit, hard-clipped", repeatRune('가', 80), repeatRune('가', shortTitleMaxRunes) + "…"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := shortTitleOf(c.goal); got != c.want {
				t.Fatalf("shortTitleOf(%q) = %q, want %q", c.goal, got, c.want)
			}
			if n := len([]rune(shortTitleOf(c.goal))); n > shortTitleMaxRunes+1 { // +1 for the "…"
				t.Fatalf("shortTitleOf(%q) = %d runes, want <= %d", c.goal, n, shortTitleMaxRunes+1)
			}
		})
	}
}

func repeatRune(r rune, n int) string {
	out := make([]rune, n)
	for i := range out {
		out[i] = r
	}
	return string(out)
}
