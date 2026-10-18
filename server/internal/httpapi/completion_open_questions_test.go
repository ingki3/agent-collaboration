package httpapi

import "testing"

// TestOpenQuestionsCountOnCompletionApproval is FR-2A.2.1f 1단계 (v0.19.30,
// Director 결정 2026-10-17): the official completion-approval card's context
// gains a visibility-only line counting whatever HITL is still open on the
// mission at the instant the approval itself opens — the real incident
// (「게임 기획」 방, 2026-10-16) had an agent-issued informal approval
// question open 10 seconds before the official one appeared, and the two
// near-identical cards confused the Director. Timing is unchanged: this does
// not touch FR-2A.2.1's hold condition (missionBusy), only the text the
// already-opened request carries.
func TestOpenQuestionsCountOnCompletionApproval(t *testing.T) {
	f := newP2Fixture(t)

	// A separate, still-open request on the SAME mission — the
	// FR-2A.2.1e "에이전트 확인 요청" kind, not the official approval.
	if _, err := f.pool.Exec(t.Context(), `
		INSERT INTO hitl_request (session_id, source, type, question, approver_spec, due_at, created_at, work_id)
		VALUES ($1, 'system', 'approval', '비공식 질문', 'director', now() + interval '1 day', now(), $2)`,
		f.sessionID, f.missionID); err != nil {
		t.Fatal(err)
	}

	hitlID := f.issueCompletionApproval(t)

	var ctx *string
	if err := f.pool.QueryRow(t.Context(), `SELECT context FROM hitl_request WHERE id = $1`, hitlID).Scan(&ctx); err != nil {
		t.Fatal(err)
	}
	if ctx == nil || !contains(*ctx, "이 미션에 아직 답 안 한 질문이 1개 있습니다") {
		t.Fatalf("context = %v, want the open-questions line", ctx)
	}

	// FR-2A.2.1 의 보류 조건은 그대로다 — an unrelated open question is not
	// "busy": the completion approval still OPENED, it was not held.
	var status string
	if err := f.pool.QueryRow(t.Context(), `SELECT status::text FROM hitl_request WHERE id = $1`, hitlID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "open" {
		t.Fatalf("completion approval status = %q, want open — an unrelated open question must not hold it", status)
	}
}

// TestOpenQuestionsCountOmittedWhenZero: no stray line when nothing else is
// open on the mission.
func TestOpenQuestionsCountOmittedWhenZero(t *testing.T) {
	f := newP2Fixture(t)
	hitlID := f.issueCompletionApproval(t)

	var ctx *string
	if err := f.pool.QueryRow(t.Context(), `SELECT context FROM hitl_request WHERE id = $1`, hitlID).Scan(&ctx); err != nil {
		t.Fatal(err)
	}
	if ctx != nil && contains(*ctx, "답 안 한 질문") {
		t.Fatalf("context = %q, want no open-questions line when nothing else is open", *ctx)
	}
}
