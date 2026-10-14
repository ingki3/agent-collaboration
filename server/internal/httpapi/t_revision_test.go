package httpapi

import (
	"testing"

	"github.com/google/uuid"

	"github.com/ingki3/agent-collabortion/server/internal/router"
)

// T-REVISION (실사용 「금융 에이전트 시장 규모 추정 방」, 2026-10-14): E6-04
// keeps `artifact_submitted` true across a Director rejection — the old
// artifact still exists. That is right for the artifact, but it means the
// very next event that re-reads the tree (here: the Director nudging the
// condition, same shape as EventTasksSettled/an agent's own request) would
// re-ask "종료 조건이 모두 충족되었습니다" over the SAME unrevised work,
// whether or not anyone did anything about the rejection. Real incident:
// Lead told the Director "새 카드로 맡길 예정" and ended its turn without
// sending one — nothing caught that, and nothing stopped a stale re-ask
// either.
func TestRevisionGateBlocksReaskUntilCardAccepted(t *testing.T) {
	f := newP2Fixture(t)
	tree := and(atom("artifact_submitted", "who", "assignee"), atom("user_approval"))
	sess := f.artifactSession(t, tree)
	leadTok, leadTask := f.agentToken(t, sess, f.leadUUID, "Lead")
	f.runTask(t, leadTask)
	if st, out := f.submit(t, sess, leadTok, "report.md", "doc", []byte("v1")); st != 201 {
		t.Fatalf("submit = %d %v", st, out)
	}
	f.endTurn(t, leadTask)
	if n := f.openHitls(t, sess); n != 1 {
		t.Fatalf("open user_approval after submit+endTurn = %d, want 1", n)
	}
	var hitlID string
	if err := f.pool.QueryRow(t.Context(), `
		SELECT id::text FROM hitl_request WHERE session_id = $1 AND purpose = 'user_approval' AND status = 'open'`,
		sess).Scan(&hitlID); err != nil {
		t.Fatal(err)
	}
	f.api.must(200, "POST", f.p+"/hitl-requests/"+hitlID+"/response",
		map[string]any{"approved": false, "reason": "Low·Base·High 계산 근거를 적어주세요"}, "Idempotency-Key", uuid.NewString())
	if n := f.openHitls(t, sess); n != 0 {
		t.Fatalf("open user_approval right after rejection = %d, want 0 (E6-04)", n)
	}

	// Re-reading the tree (a Director nudge — same mechanism S-84 uses, and
	// the shape any later settle takes) must NOT re-ask over the still
	// unrevised artifact: no card has been accepted since the rejection.
	if st, out := f.patchCond(t, f.api, sess, tree); st != 200 {
		t.Fatalf("patchCond (no card yet) = %d %v", st, out)
	}
	if n := f.openHitls(t, sess); n != 0 {
		t.Fatalf("open user_approval after a rejection with no accepted card yet = %d, want 0 — the revision was not addressed", n)
	}

	// Lead delegates the revision, Writer finishes it, the Director accepts
	// it — THAT is what "addressed" means.
	_, lead2 := f.agentToken(t, sess, f.leadUUID, "Lead")
	f.runTask(t, lead2)
	res, err := f.srv.Router.Delegate(t.Context(), lead2, testCard(f.wUUID, "report.md 에 계산 근거 추가"))
	if err != nil {
		t.Fatal(err)
	}
	childTask := uuid.UUID(res.Task.Id)
	f.runTask(t, childTask)
	f.report(t, childTask)
	f.finishCompleted(t, childTask)
	var dir uuid.UUID
	if err := f.pool.QueryRow(t.Context(), `SELECT id FROM app_user WHERE email = 'dir@example.com'`).Scan(&dir); err != nil {
		t.Fatal(err)
	}
	if _, err := f.srv.Router.Accept(t.Context(), res.Card.Id, router.Judgement{UserID: &dir}, "근거 확인함"); err != nil {
		t.Fatal(err)
	}
	// Lead's delegating turn (lead2) is still "running", and the accept
	// itself wakes Lead with a notification trigger — settle everything so
	// the mission-busy check (T-APPROVAL, independent of this gate) does not
	// itself hold the re-ask.
	f.endTurn(t, lead2)
	f.settleWork(t, f.missionOf(t, sess))

	// The same nudge now releases the gate — the revision has evidence.
	if st, out := f.patchCond(t, f.api, sess, tree); st != 200 {
		t.Fatalf("patchCond (card accepted) = %d %v", st, out)
	}
	if n := f.openHitls(t, sess); n != 1 {
		t.Fatalf("open user_approval after the revision card was accepted = %d, want 1 — the gate should release now", n)
	}
}

// A mission that has never been rejected is not gated by T-REVISION at all —
// the existing T-APPROVAL path (t_approval_test.go) is untouched.
func TestRevisionGateNoOpWithoutPriorRejection(t *testing.T) {
	f := newP2Fixture(t)
	sess := f.artifactSession(t, and(atom("artifact_submitted", "who", "assignee"), atom("user_approval")))
	leadTok, leadTask := f.agentToken(t, sess, f.leadUUID, "Lead")
	f.runTask(t, leadTask)
	if st, out := f.submit(t, sess, leadTok, "a.md", "doc", []byte("a")); st != 201 {
		t.Fatalf("submit = %d %v", st, out)
	}
	f.endTurn(t, leadTask)
	if n := f.openHitls(t, sess); n != 1 {
		t.Fatalf("open user_approval with no prior rejection = %d, want 1 — T-REVISION must not gate a first ask", n)
	}
}
