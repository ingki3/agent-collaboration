package httpapi

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ingki3/agent-collabortion/contracts"
	"github.com/ingki3/agent-collabortion/server/internal/httpapi/gen"
	"github.com/ingki3/agent-collabortion/server/internal/router"
	"github.com/ingki3/agent-collabortion/server/internal/sessions"
)

// FR-1.3b (Director 결정 2026-10-13, real incident — 「1인 창업 사업 계획서
// 작성」 방): Lead asked a `choice` question (its task went `waiting_human`,
// daemon-protocol §4.4 — the server reads `pending_hitl` on finish, not the
// daemon). The Director answered with a plain chat mention instead of the
// card's own "답변" button; Lead read that and kept working for 30+ minutes
// in the SAME lane. The asking task never left `waiting_human` — nobody ever
// called respondHitlRequest on it — and sessions.AgentStatuses's derivation
// was a bare `EXISTS … status = 'waiting_human'` with no regard for whether
// a newer task in the same lane already superseded it, so the roster chip
// kept reading "승인 대기" long after there was nothing left to approve.
//
// The fix is read-side only: a waiting_human task drives the chip while it
// is its own lane's MOST RECENT task. The task and its hitl_request are
// never touched — FR-1.9 M8's kill-switch guarantee ("a person's unanswered
// question is never the agent's to lose") stays intact, verified separately
// by TestP3KillSwitch.
func TestRosterChipIgnoresWaitingHumanSupersededInItsOwnLane(t *testing.T) {
	f := newP2Fixture(t)
	tok, taskID := f.agentToken(t, f.sessionID, f.leadUUID, "Lead")
	f.runTask(t, taskID)
	f.hitlOn(t, tok, map[string]any{
		"type": "choice", "question": "사업계획서 아이템을 무엇으로 쓸까요?",
		"options": []string{"E→A", "A만", "B"}, "proposed_default": "E→A",
	}, 201)

	// The asking task's own finish lands on waiting_human — the server
	// overrides `completed` because PendingHitl is set (tasks.Service.Finish).
	if _, err := f.srv.Tasks.Finish(t.Context(), taskID, currentAttempt(t, f, taskID), contracts.Finish{
		Outcome: "completed", StopReason: "end_turn",
	}); err != nil {
		t.Fatal(err)
	}
	if got := f.taskStatus(t, taskID); got != "waiting_human" {
		t.Fatalf("asking task status = %s, want waiting_human", got)
	}

	roomID := mustUUID(t, f.sessionID)
	before, err := sessions.AgentStatuses(t.Context(), f.pool, roomID, &f.leadUUID)
	if err != nil {
		t.Fatal(err)
	}
	if got := before[f.leadUUID]; got != gen.AgentStatusWaitingHuman {
		t.Fatalf("chip right after the ask = %q, want waiting_human — this is the ONE live task in the lane, correctly reflected", got)
	}

	// The Director answers in plain chat instead of the card's "답변" button
	// — same mention shape agentToken used the first time, reusing the lane
	// (lane rule 3: reentry — waiting_human is the real incident's shape).
	f.fake.Advance(time.Minute)
	out := f.api.must(201, "POST", f.p+"/rooms/"+f.sessionID+"/messages",
		map[string]any{"content": router.MentionLink("Lead", f.leadUUID) + " 제안한 대로 작성 진행해봐"},
		"Idempotency-Key", uuid.NewString())
	var newTask uuid.UUID
	for _, raw := range out["triggers"].([]any) {
		tr := raw.(map[string]any)
		if str(tr, "agent_id") == f.leadUUID.String() {
			newTask = mustUUID(t, str(tr, "task_id"))
		}
	}
	if newTask == uuid.Nil {
		t.Fatalf("no new task for Lead: %v", out["triggers"])
	}

	// Still open — the fix does not touch the request itself (FR-1.9 M8).
	if got := f.taskStatus(t, taskID); got != "waiting_human" {
		t.Fatalf("the original asking task's own status = %s, want still waiting_human (untouched)", got)
	}

	// The new task is only `queued` (never claimed), so the ladder's next
	// rung after waiting_human is idle — what matters is that the SUPERSEDED
	// task no longer drives the chip.
	after, err := sessions.AgentStatuses(t.Context(), f.pool, roomID, &f.leadUUID)
	if err != nil {
		t.Fatal(err)
	}
	if got := after[f.leadUUID]; got != gen.AgentStatusIdle {
		t.Fatalf("chip after a newer task exists in the same lane = %q, want idle (not waiting_human — nothing is actually pending on the Director any more)", got)
	}

	// Claiming the new task flips the ladder to working, same lane.
	f.runTask(t, newTask)
	afterRunning, err := sessions.AgentStatuses(t.Context(), f.pool, roomID, &f.leadUUID)
	if err != nil {
		t.Fatal(err)
	}
	if got := afterRunning[f.leadUUID]; got != gen.AgentStatusWorking {
		t.Fatalf("chip once the new task runs = %q, want working", got)
	}
}
