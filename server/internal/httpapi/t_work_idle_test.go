package httpapi

import "testing"

// workIDOf is the work id of a session created by artifactSession (one
// mission per room there, newest first — same lookup p5_reviewer_required_test.go uses).
func (f *p2Fixture) workIDOf(t *testing.T, sess string) string {
	t.Helper()
	var id string
	if err := f.pool.QueryRow(t.Context(),
		`SELECT id::text FROM work WHERE room_id = $1 ORDER BY created_at LIMIT 1`, sess).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// FR-2A.2.1b (Director 결정 2026-10-13, 실사용 「1인 창업 사업 계획서 작성」
// 방): a mission whose completion tree is `user_approval` alone never gets a
// qualifying event (no artifact_submitted/agent_approval atom for anyone to
// satisfy) — ApplyWorkEvent never runs once, so the platform's approval
// request sits unraised forever even though every lane has long settled.
// NotifyMissionIdle is the safety net: it only reads the tree to word a
// notice, never mutates completion state or opens a hitl_request.

// workIdleCount is the work_idle inbox item count for this mission's own
// Director — joined through member so the test does not need their user id.
func (f *p2Fixture) workIdleCount(t *testing.T, workID string) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(t.Context(), `
		SELECT count(*) FROM inbox_item i JOIN member m ON m.id = i.member_id
		JOIN work w ON w.director_user_id = m.user_id
		WHERE i.work_id = $1 AND i.type = 'work_idle' AND w.id = $1`, workID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (f *p2Fixture) idleNoticeCount(t *testing.T, sessionID string) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(t.Context(), `
		SELECT count(*) FROM message WHERE session_id = $1 AND author_type = 'system'
		AND content LIKE '%조용해졌습니다%'`, sessionID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestWorkIdleNoticeWhenApprovalNeverAsked(t *testing.T) {
	f := newP2Fixture(t)
	// No artifact_submitted atom at all — the real incident's shape. Nothing
	// anyone can submit or review ever makes ApplyWorkEvent run.
	sess := f.artifactSession(t, and(atom("user_approval")))
	workID := f.workIDOf(t, sess)
	_, leadTask := f.agentToken(t, sess, f.leadUUID, "Lead")

	// The turn ends with no artifact submit, no review, no condition change —
	// exactly the gap: the formal approval flow never had a reason to run.
	f.endTurn(t, leadTask)

	if n := f.openHitls(t, sess); n != 0 {
		t.Fatalf("open hitl_request after settle with no qualifying event = %d, want 0 — confirms the gap this hook covers", n)
	}
	if n := f.idleNoticeCount(t, sess); n != 1 {
		t.Fatalf("system notice mentioning 조용해졌습니다 = %d, want exactly 1", n)
	}
	if n := f.workIdleCount(t, workID); n != 1 {
		t.Fatalf("work_idle inbox items = %d, want exactly 1", n)
	}

	// A second settle (e.g. the daemon's repeated finish, or the scheduler's
	// catch-all calling this again) must not stack a duplicate while the
	// first is unread.
	fired, err := f.srv.Sessions.NotifyMissionIdle(t.Context(), mustUUID(t, workID))
	if err != nil {
		t.Fatal(err)
	}
	if fired {
		t.Fatalf("NotifyMissionIdle fired again with an unread work_idle item already open")
	}
	if n := f.workIdleCount(t, workID); n != 1 {
		t.Fatalf("work_idle inbox items after a second settle = %d, want still 1", n)
	}
}

// TestWorkIdleSkipsWhileFormalApprovalFlowApplies is the Director's explicit
// requirement: this notice must stay out of the way whenever FR-2A.2.1's own
// held/open machinery is already handling the mission — held while work ran,
// then an open user_approval request once it is released.
func TestWorkIdleSkipsWhileFormalApprovalFlowApplies(t *testing.T) {
	f := newP2Fixture(t)
	sess := f.artifactSession(t, and(atom("artifact_submitted", "who", "assignee"), atom("user_approval")))
	workID := f.workIDOf(t, sess)
	leadTok, leadTask := f.agentToken(t, sess, f.leadUUID, "Lead")
	f.runTask(t, leadTask)

	// Submitted mid-turn: the tree has only user_approval left, so the
	// request is HELD (approval_held_at set) rather than opened.
	if st, out := f.submit(t, sess, leadTok, "plan.md", "doc", []byte("v1")); st != 201 {
		t.Fatalf("submit = %d %v", st, out)
	}
	if n := f.workIdleCount(t, workID); n != 0 {
		t.Fatalf("work_idle while held = %d, want 0 — FR-2A.2.1's hold already covers this", n)
	}

	// The turn ends: ReleaseHeldApproval opens the real request. The safety
	// net must see that open request and stay out, not pile a second notice
	// on top of the formal card the Director already has.
	f.endTurn(t, leadTask)
	if n := f.openHitls(t, sess); n != 1 {
		t.Fatalf("open user_approval after settle = %d, want 1", n)
	}
	if n := f.workIdleCount(t, workID); n != 0 {
		t.Fatalf("work_idle after the formal request opened = %d, want 0 — one card, not two, for the same thing (FR-2A.2.2's own principle)", n)
	}

	fired, err := f.srv.Sessions.NotifyMissionIdle(t.Context(), mustUUID(t, workID))
	if err != nil {
		t.Fatal(err)
	}
	if fired {
		t.Fatalf("NotifyMissionIdle fired while a formal user_approval request is open")
	}
}
