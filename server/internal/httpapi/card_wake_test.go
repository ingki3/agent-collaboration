package httpapi

// T-CARD-WAKE: regression for 실사용 사고 2026-11-06/07 (room f7be9f73, 최대
// 10시간 두 번 방치) — cards.StoreResult writes the result bubble with a
// direct INSERT that never reaches Decide()/routeTrigger, so submitting a
// result card created no task for the delegator by itself. Three pieces,
// each covered here:
//
//   - router/cards.go SubmitResult now calls wakeDelegatorForResult right
//     after StoreResult: idle delegator → force-wake
//     (TestCardSubmitWakesDelegator); already running → attach only, no
//     extra hop (TestCardSubmitDoesNotDoubleWakeARunningDelegator).
//   - router/status.go maybeFireJoin and wakeDelegatorForResult share
//     activeTaskFor so that an idle-wake followed moments later by the same
//     group's join does not hop the one round-trip twice
//     (TestCardSubmitDoesNotDoubleWakeWhenJoinFollows — an earlier version
//     of this fix missed this interaction, confirmed via a direct
//     session_hop count).
//   - router/status.go notifyReentry's requesterAlreadyWoken gate used to
//     skip attaching a still-unjudged result card whenever the lane had
//     ALREADY woken the delegator for anything else, which an ordinary
//     courtesy mention always satisfied — reachable only on a REVISE
//     re-entry (TestCardResultSurvivesPriorWakeOnReentry).
import (
	"testing"

	"github.com/google/uuid"

	"github.com/ingki3/agent-collaboration/server/internal/cards"
	"github.com/ingki3/agent-collaboration/server/internal/httpapi/gen"
	"github.com/ingki3/agent-collaboration/server/internal/router"
)

// resultCardIDsOn is a task's <result_cards> payload (attachResultCards).
func (f *p2Fixture) resultCardIDsOn(t *testing.T, taskID uuid.UUID) []uuid.UUID {
	t.Helper()
	var ids []uuid.UUID
	if err := f.pool.QueryRow(t.Context(), `SELECT result_card_ids FROM task WHERE id = $1`, taskID).Scan(&ids); err != nil {
		t.Fatal(err)
	}
	return ids
}

// (card-submit-wake) a result submitted while the delegator is IDLE (its own
// delegating turn already ended, R's lane is not finished yet either) must
// still force a fresh wake — nothing else is ever going to tell it.
func TestCardSubmitWakesDelegator(t *testing.T) {
	f := newP2Fixture(t)
	leadTask, rTask, rLane := f.delegatedChild(t)
	// Lead's own delegating turn ends (idle: no running/queued task of its
	// own) — leaving ONLY the lane-end and submission-time paths able to
	// reach it, which is what this test isolates.
	if _, err := f.pool.Exec(t.Context(), `UPDATE task SET status = 'completed' WHERE id = $1`, leadTask); err != nil {
		t.Fatal(err)
	}
	before := f.count(t, `SELECT count(*) FROM task WHERE agent_id = $1 AND status = 'queued'`, f.leadUUID)
	if before != 0 {
		t.Fatalf("setup: lead queued tasks = %d, want 0 (idle)", before)
	}

	f.report(t, rTask) // submits the result card; R's lane is NOT finished

	if st := f.laneStatus(t, rLane); st == "done" {
		t.Fatalf("setup: r lane = %s, want not done (this tests the mid-lane case)", st)
	}
	after := f.count(t, `SELECT count(*) FROM task WHERE agent_id = $1 AND status = 'queued'`, f.leadUUID)
	if after != 1 {
		t.Fatalf("(card-submit-wake) lead queued tasks after report = %d, want 1", after)
	}
	leadQueued, _, _ := f.queuedOnLane(t, f.laneOfTask(t, leadTask))
	card := f.taskCard(t, rTask)
	if card.Status != cards.ResultSubmitted {
		t.Fatalf("card status = %s, want result_submitted", card.Status)
	}
	ids := f.resultCardIDsOn(t, leadQueued)
	if len(ids) != 1 || ids[0] != card.ID {
		t.Fatalf("(card-submit-wake) lead's queued task result_card_ids = %v, want [%s]", ids, card.ID)
	}
}

// (card-submit-no-double-hop) when the delegator ALREADY has a live task
// (running, as it normally does mid-mission), a result submission must NOT
// force a second wake/hop — it only attaches the card, trusting the
// already-active task or the lane's eventual end to carry it.
func TestCardSubmitDoesNotDoubleWakeARunningDelegator(t *testing.T) {
	f := newP2Fixture(t)
	leadTask, rTask, _ := f.delegatedChild(t) // Lead's own task is left "running"

	f.report(t, rTask)

	if n := f.count(t, `SELECT count(*) FROM task WHERE agent_id = $1 AND status = 'queued'`, f.leadUUID); n != 0 {
		t.Fatalf("(card-submit-no-double-hop) lead queued tasks = %d, want 0 (its running task is trusted, not re-woken)", n)
	}
	// The card still exists, correctly unjudged — just not yet pushed onto Lead.
	card := f.taskCard(t, rTask)
	if card.Status != cards.ResultSubmitted {
		t.Fatalf("card status = %s, want result_submitted", card.Status)
	}
	_ = leadTask
}

// (card-no-double-hop-on-join) reproduces the exact 코드리뷰 2026-11-07
// regression (confirmed by a direct session_hop count): an idle delegator
// whose only child reports a card and ends its turn right after must get
// exactly ONE FR-3.5 hop for that round-trip, not one from
// wakeDelegatorForResult's idle wake and a second from maybeFireJoin finding
// the same, now-complete group moments later and re-waking on top of it.
func TestCardSubmitDoesNotDoubleWakeWhenJoinFollows(t *testing.T) {
	f := newP2Fixture(t)
	leadTask, rTask, _ := f.delegatedChild(t)
	if _, err := f.pool.Exec(t.Context(), `UPDATE task SET status = 'completed' WHERE id = $1`, leadTask); err != nil {
		t.Fatal(err)
	}

	f.finishCompleted(t, rTask) // report (idle wake) then end R's lane (join fires)

	if n := f.count(t, `SELECT count(*) FROM task WHERE agent_id = $1 AND status = 'queued'`, f.leadUUID); n != 1 {
		t.Fatalf("(card-no-double-hop-on-join) lead queued tasks = %d, want 1 (one wake-up, not two)", n)
	}
	id, _, _ := f.queuedOnLane(t, f.laneOfTask(t, leadTask))
	card := f.taskCard(t, rTask)
	if ids := f.resultCardIDsOn(t, id); len(ids) != 1 || ids[0] != card.ID {
		t.Fatalf("result_card_ids = %v, want [%s]", ids, card.ID)
	}
	if n := f.count(t, `SELECT count(*) FROM session_hop WHERE session_id = $1 AND from_agent_id = $2 AND to_agent_id = $3`,
		mustUUID(t, f.sessionID), f.rUUID, f.leadUUID); n != 1 {
		t.Fatalf("(card-no-double-hop-on-join) R→Lead session_hop rows = %d, want 1", n)
	}
}

// (card-wake-not-swallowed) notifyReentry's requesterAlreadyWoken gate must
// not swallow a still-unjudged card. The only path that actually reaches
// that branch is a REVISE re-entry (afterLaneDone only calls notifyReentry
// when reentry>0; a fresh delegation's lane always has delegated_from_task_id
// set, so afterLaneDone routes it to maybeFireJoin instead — see that
// function's own test above). This mirrors TestCardJudgeFlow's own
// two-child-then-revise shape up to the join firing, then reproduces the
// production sequence on the re-entry: R's re-entry posts a courtesy mention
// to Lead (legitimately satisfying requesterAlreadyWoken — and, realistically,
// that small task runs to completion on its own) before the new result lands
// and the re-entry's turn ends.
func TestCardResultSurvivesPriorWakeOnReentry(t *testing.T) {
	f := newP2Fixture(t)
	leadTask, c1, c2 := f.twoChildren(t)
	f.report(t, c1)
	f.finishCompleted(t, c1)
	f.report(t, c2)
	f.finishCompleted(t, c2)
	if !joinFired(t, f, leadTask) {
		t.Fatal("setup: join did not fire")
	}
	leadLane := f.laneOfTask(t, leadTask)
	leadNext, _, _ := f.queuedOnLane(t, leadLane)
	if _, err := f.pool.Exec(t.Context(), `UPDATE task SET status = 'completed' WHERE id = $1`, leadTask); err != nil {
		t.Fatal(err)
	}
	f.runTask(t, leadNext) // marks leadNext running (claimBundle also completes it — wrong here)

	card2 := f.taskCard(t, c2)
	goal := "B 조사 — 출처를 붙여서"
	rv, err := f.srv.Router.Revise(t.Context(), card2.ID, router.Judgement{TaskID: &leadNext, Attempt: 1}, "출처가 없습니다", router.Patch{Goal: &goal})
	if err != nil {
		t.Fatal(err)
	}
	re := uuid.UUID(rv.Task.Id)
	f.runTask(t, re) // claimed: running

	author := router.Author{Type: "agent", AgentID: &f.rUUID, TaskID: &re, Attempt: 1}
	trig, err := f.srv.Router.Post(t.Context(), mustUUID(t, f.sessionID), author,
		gen.MessageCreate{Content: router.MentionLink("Lead", f.leadUUID) + " 수정 중입니다"})
	if err != nil {
		t.Fatal(err)
	}
	if len(trig.Triggers) != 1 {
		t.Fatalf("setup: courtesy mention triggers = %v, want exactly Lead", trig.Triggers)
	}
	if _, err := f.pool.Exec(t.Context(), `UPDATE task SET status = 'completed' WHERE id = $1`, uuid.UUID(trig.Triggers[0].TaskId)); err != nil {
		t.Fatal(err)
	}
	// Precondition: Lead's only ACTIVE task right now is leadNext (running) —
	// this is what keeps wakeDelegatorForResult out of the way (its "running"
	// branch) below, isolating notifyReentry's own bypass.
	if n := f.count(t, `SELECT count(*) FROM task WHERE agent_id = $1 AND status IN ('running', 'queued')`, f.leadUUID); n != 1 {
		t.Fatalf("setup: lead active tasks = %d, want 1 (leadNext only)", n)
	}

	f.finishCompleted(t, re) // reports v2, then ends R's re-entry (reentry_count+1)

	card := f.taskCard(t, c2)
	if card.Status != cards.ResultSubmitted || card.Version != 2 {
		t.Fatalf("card = %s v%d, want result_submitted v2", card.Status, card.Version)
	}
	id, _, _ := f.queuedOnLane(t, leadLane)
	ids := f.resultCardIDsOn(t, id)
	if len(ids) != 1 || ids[0] != card.ID {
		t.Fatalf("(card-wake-not-swallowed) lead's queued task result_card_ids = %v, want [%s] — "+
			"a prior unrelated wake must not let notifyReentry skip the still-unjudged card on a revise re-entry", ids, card.ID)
	}
}
