package queue

// T-SHARELINK (harness v0.9.21, PRD v0.19.29 FR-4.3.2): brief [2] carries a
// fixed line telling the agent to give a person the artifact's `share_url`
// (not the API path, which a person's browser cannot open behind a reverse
// proxy or tunnel — 실사용 사고 「게임 기획」 방, 2026-10-16).
//
// 회귀 주입: Section2 에서 s.ShareLinkRule 을 빼면 (brief2) FAIL; ShareLinkRule
// 에서 "share_url" 을 지우면 (contract) FAIL; mcpSurface 의 ShareLinkRule 을
// 셸 문장으로 두면 (surface) FAIL.

import (
	"regexp"
	"strings"
	"testing"
)

func contractV0921(t *testing.T) (shell, mcp string) {
	t.Helper()
	c := harnessContract(t)
	m := regexp.MustCompile("v0\\.9\\.21 — [^|]*?셸 `([^`]+)` / mcp `([^`]+)`").FindStringSubmatch(c)
	if m == nil {
		t.Fatal("harness.md has no v0.9.21 brief [2] shell/mcp sentences")
	}
	return m[1], m[2]
}

func TestShareLinkRuleMatchesContractV0921(t *testing.T) {
	shell, mcp := contractV0921(t)

	// (contract) the shell line is the contract's sentence, word for word
	// (backticks aside — the brief puts share_url and the command in
	// backticks, the contract's 「…」 quote does not).
	got := strings.ReplaceAll(strings.TrimSuffix(strings.TrimPrefix(ShareLinkRule, "- "), "\n"), "`", "")
	if got != shell {
		t.Errorf("(contract) shell line differs from harness v0.9.21:\n got %q\nwant %q", got, shell)
	}
	if !strings.Contains(ShareLinkRule, "share_url") {
		t.Errorf("(contract) shell line lost share_url: %q", ShareLinkRule)
	}

	// (contract) the mcp line: the contract's tool phrase, and the same head
	// and tail as the shell one (the contract's 「…」).
	parts := strings.Split(mcp, "…")
	if len(parts) < 3 {
		t.Fatalf("contract mcp quote changed shape: %q", mcp)
	}
	tool := strings.TrimSpace(parts[1])
	if !strings.Contains(tool, "colab_artifact_get") || !strings.Contains(ShareLinkRuleMCP, "colab_artifact_get") {
		t.Errorf("(contract) mcp line lacks the tool names\n  contract %q\n  line     %q", tool, ShareLinkRuleMCP)
	}
	head := shell[:strings.Index(shell, "use its share_url")]
	mcpStripped := strings.ReplaceAll(ShareLinkRuleMCP, "`", "")
	if !strings.HasPrefix(mcpStripped, "- "+head+"use its share_url") {
		t.Errorf("(contract) mcp line does not start like the shell one: %q", ShareLinkRuleMCP)
	}
	tail := shell[strings.LastIndex(shell, "— not the API path"):]
	if !strings.HasSuffix(strings.TrimSuffix(got, "\n"), tail) || !strings.HasSuffix(strings.TrimSuffix(mcpStripped, "\n"), tail) {
		t.Errorf("(contract) a line lost the proxy/tunnel tail")
	}

	// (surface) mcp names no shell command; shell names no tool.
	if strings.Contains(ShareLinkRuleMCP, "`colab ") {
		t.Errorf("(surface) mcp line names the shell command: %q", ShareLinkRuleMCP)
	}
	if strings.Contains(ShareLinkRule, "colab_") {
		t.Errorf("(surface) shell line names an MCP tool: %q", ShareLinkRule)
	}

	// (brief2) both surfaces carry it in [2], once, right after the media line.
	for kind, s := range map[string]Surface{"mcp": SurfaceFor("claude_code"), "cli_wrapper": SurfaceFor("hermes")} {
		if n := strings.Count(s.Section2(), s.ShareLinkRule); s.ShareLinkRule == "" || n != 1 {
			t.Errorf("(brief2) %s [2] carries the share-link line %d times:\n%s", kind, n, s.Section2())
		}
		if !strings.Contains(s.Section2(), s.MediaRule+"\n"+s.ShareLinkRule) {
			t.Errorf("(brief2) %s [2] does not put the share-link line right after the media line", kind)
		}
	}
	if SurfaceFor("claude_code").ShareLinkRule != ShareLinkRuleMCP || SurfaceFor("hermes").ShareLinkRule != ShareLinkRule {
		t.Error("(surface) SurfaceFor picked the wrong share-link line")
	}
}
