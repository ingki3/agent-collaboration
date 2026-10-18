package httpapi

import (
	"strings"
	"testing"
)

// TestEndorseArtifactRealPath is FR-2A.1a's (v0.19.28) real path: W submits,
// but the tree names Lead (the mission's assignee, an orchestrator role that
// does not submit directly) — Lead endorsing W's artifact is the SAME kind of
// event as Lead submitting it themselves (actor = the endorsing agent).
func TestEndorseArtifactRealPath(t *testing.T) {
	f := newP2Fixture(t)
	sess := f.artifactSession(t, map[string]any{"op": "and", "conditions": []map[string]any{
		{"type": "artifact_submitted", "who": "assignee"}, {"type": "user_approval"},
	}})
	wTok, wTask := f.agentToken(t, sess, f.wUUID, "W")
	leadTok, leadTask := f.agentToken(t, sess, f.leadUUID, "Lead")

	st, out := f.submit(t, sess, wTok, "draft.md", "doc", []byte("초안"))
	if st != 201 {
		t.Fatalf("submit = %d: %v", st, out)
	}
	id := str(out["artifact"].(map[string]any), "id")
	prog := out["completion_progress"].(map[string]any)
	if met := int(prog["met"].(float64)); met != 0 {
		t.Fatalf("met = %d, want 0 — W is not the designated assignee", met)
	}

	// Lead endorses it as their own submission.
	st, out = f.rawPost(t, f.p+"/artifacts/"+id+"/endorsements", leadTok, map[string]any{"comment": "확인했고 이걸로 갑니다"})
	if st != 200 {
		t.Fatalf("endorse = %d, want 200: %v", st, out)
	}
	prog = out["completion_progress"].(map[string]any)
	if met := int(prog["met"].(float64)); met != 1 {
		t.Fatalf("met = %d, want 1 (artifact_submitted) after the designated agent endorses", met)
	}
	end := out["endorsement"].(map[string]any)
	if str(end, "artifact_id") != id || str(end, "endorsed_by_agent_id") != f.lead {
		t.Fatalf("endorsement = %v", end)
	}
	if c, _ := end["comment"].(string); c != "확인했고 이걸로 갑니다" {
		t.Fatalf("endorsement comment = %q", c)
	}

	// No resubmission happened — still exactly one artifact row.
	var artifacts, endorsements int
	if err := f.pool.QueryRow(t.Context(), `SELECT count(*) FROM artifact WHERE session_id = $1`, mustUUID(t, sess)).Scan(&artifacts); err != nil {
		t.Fatal(err)
	}
	if artifacts != 1 {
		t.Fatalf("stored %d artifacts, want 1 — endorsing is not resubmitting", artifacts)
	}
	if err := f.pool.QueryRow(t.Context(), `SELECT count(*) FROM artifact_endorsement WHERE artifact_id = $1`, mustUUID(t, id)).Scan(&endorsements); err != nil {
		t.Fatal(err)
	}
	if endorsements != 1 {
		t.Fatalf("stored %d endorsements, want 1", endorsements)
	}

	// Both turns end — the platform issues the now-due user_approval HITL,
	// same as a direct submit would (T-APPROVAL).
	f.endTurn(t, wTask)
	f.endTurn(t, leadTask)
	var hitls int
	if err := f.pool.QueryRow(t.Context(),
		`SELECT count(*) FROM hitl_request WHERE session_id = $1 AND source = 'system'`, mustUUID(t, sess)).Scan(&hitls); err != nil {
		t.Fatal(err)
	}
	if hitls != 1 {
		t.Fatalf("system HITLs = %d, want 1", hitls)
	}
}

// TestEndorseByNonDesignatedAgentIsRefused mirrors ReviewArtifact's
// designation gate (not_designated_reviewer) with endorse's own code
// (not_designated_assignee) — refused before anything is stored.
func TestEndorseByNonDesignatedAgentIsRefused(t *testing.T) {
	f := newP2Fixture(t)
	sess := f.artifactSession(t, map[string]any{"op": "and", "conditions": []map[string]any{
		{"type": "artifact_submitted", "who": "assignee"}, {"type": "user_approval"},
	}})
	wTok, _ := f.agentToken(t, sess, f.wUUID, "W")
	rTok, _ := f.agentToken(t, sess, f.rUUID, "R")

	st, out := f.submit(t, sess, wTok, "draft.md", "doc", []byte("초안"))
	if st != 201 {
		t.Fatalf("submit = %d: %v", st, out)
	}
	id := str(out["artifact"].(map[string]any), "id")

	st, out = f.rawPost(t, f.p+"/artifacts/"+id+"/endorsements", rTok, map[string]any{"comment": "제가 대신"})
	if st != 403 {
		t.Fatalf("non-assignee endorse = %d, want 403", st)
	}
	if code := str(out, "code"); code != "not_designated_assignee" {
		t.Fatalf("403 code = %q, want not_designated_assignee", code)
	}
	var n int
	if err := f.pool.QueryRow(t.Context(), `SELECT count(*) FROM artifact_endorsement WHERE artifact_id = $1`, mustUUID(t, id)).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("a refused endorse stored %d rows, want 0", n)
	}
	if got := f.sessionStatus(t, sess); got != "active" {
		t.Fatalf("session = %q after a refused endorse, want active", got)
	}
}

// TestEndorseCommentRequired is v0.19.33's correction: the designation gate
// passing is not enough — an empty, whitespace-only, or invisible-runes-only
// comment is 422, the exact same shape acceptCard's judgement_comment_required
// uses (cards.JudgementText), so a 1-field button cannot be clicked without
// having looked at what it endorses. Over the 600-rune cap is too_long.
func TestEndorseCommentRequired(t *testing.T) {
	f := newP2Fixture(t)
	sess := f.artifactSession(t, map[string]any{"op": "and", "conditions": []map[string]any{
		{"type": "artifact_submitted", "who": "assignee"}, {"type": "user_approval"},
	}})
	wTok, _ := f.agentToken(t, sess, f.wUUID, "W")
	leadTok, _ := f.agentToken(t, sess, f.leadUUID, "Lead")

	st, out := f.submit(t, sess, wTok, "draft.md", "doc", []byte("초안"))
	if st != 201 {
		t.Fatalf("submit = %d: %v", st, out)
	}
	id := str(out["artifact"].(map[string]any), "id")

	for _, c := range []string{"", "   ", "​ㅤ"} {
		st, out := f.rawPost(t, f.p+"/artifacts/"+id+"/endorsements", leadTok, map[string]any{"comment": c})
		if st != 422 {
			t.Fatalf("endorse with comment %q = %d, want 422: %v", c, st, out)
		}
		if code := str(out, "code"); code != "endorse_comment_required" {
			t.Fatalf("422 code = %q, want endorse_comment_required (comment %q)", code, c)
		}
	}
	// No body at all (JSON decode skipped entirely) must answer the same 422,
	// not a decode error — the body is optional in shape, the comment is not.
	st, out = f.raw(t, "POST", f.p+"/artifacts/"+id+"/endorsements", leadTok, "", nil)
	if st != 422 {
		t.Fatalf("endorse with no body = %d, want 422: %v", st, out)
	}

	long := strings.Repeat("a", 601)
	st, out = f.rawPost(t, f.p+"/artifacts/"+id+"/endorsements", leadTok, map[string]any{"comment": long})
	if st != 422 {
		t.Fatalf("endorse with 601-char comment = %d, want 422: %v", st, out)
	}
	errs, _ := out["errors"].([]any)
	if len(errs) != 1 || str(errs[0].(map[string]any), "code") != "too_long" {
		t.Fatalf("errors = %v, want one field error code too_long", out["errors"])
	}

	var n int
	if err := f.pool.QueryRow(t.Context(), `SELECT count(*) FROM artifact_endorsement WHERE artifact_id = $1`, mustUUID(t, id)).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("a refused (comment) endorse stored %d rows, want 0", n)
	}
}

// TestEndorseIsAgentOnly: endorseArtifact's only security scheme is TaskToken.
func TestEndorseIsAgentOnly(t *testing.T) {
	f := newP2Fixture(t)
	sess := f.artifactSession(t, map[string]any{"op": "and", "conditions": []map[string]any{
		{"type": "artifact_submitted", "who": "assignee"}, {"type": "user_approval"},
	}})
	tok, _ := f.agentToken(t, sess, f.leadUUID, "Lead")
	st, out := f.submit(t, sess, tok, "d.md", "doc", []byte("d"))
	if st != 201 {
		t.Fatalf("submit = %d: %v", st, out)
	}
	id := str(out["artifact"].(map[string]any), "id")
	if st, _, _ := f.api.do("POST", f.p+"/artifacts/"+id+"/endorsements", map[string]any{"comment": "x"}); st != 403 {
		t.Fatalf("a person calling endorseArtifact = %d, want 403 agent_only", st)
	}
}
