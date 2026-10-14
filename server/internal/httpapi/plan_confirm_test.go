package httpapi

// FR-2A.7 (v1.2, 0049, 실사용 요청 2026-10-14): a mission the Director opted
// into plan confirmation blocks every delegation until an approved
// `purpose: plan` HITL lifts it — the point is that nothing is handed to a
// teammate before the Director has seen and confirmed the Lead's
// understanding of the mission.
//
// 회귀 주입 (각 규칙을 끄면 아래 이름의 단언이 FAIL):
//   - router.Delegate 의 plan_confirm_required 검사 삭제  → (gate)
//   - createHitl 의 미션 상태 검사(not required/already confirmed) 삭제 → (not-applicable)
//   - hitl.PlanRegister 의 PlanConfirm kind 검사 삭제      → (approval-only)
//   - answerAgentHitl 의 plan_confirmed_at 갱신 삭제       → (gate-lifts)

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ingki3/agent-collabortion/server/internal/router"
	"github.com/ingki3/agent-collabortion/server/internal/tokens"
)

func TestPlanConfirmGate(t *testing.T) {
	f := newP2Fixture(t)
	ctx := t.Context()

	// (not-applicable) plan_confirm on a mission that never opted in.
	tok0, task0 := f.agentToken(t, f.sessionID, f.leadUUID, "Lead")
	f.hitlOn(t, tok0, map[string]any{
		"type": "approval", "summary": "계획 확인", "context": "목표·접근법", "plan_confirm": true,
	}, 422)
	_ = task0

	// A mission the Director opened WITH plan confirmation required.
	wB := str(f.api.must(201, "POST", f.p+"/rooms/"+f.sessionID+"/works", map[string]any{
		"title": "계획 확인 미션", "goal": "계획 확인이 필요한 미션", "assignee_agent_id": f.lead,
		"plan_confirm_required": true,
	}), "id")
	var required bool
	var confirmedAt *time.Time
	if err := f.pool.QueryRow(ctx, `SELECT plan_confirm_required, plan_confirmed_at FROM work WHERE id = $1`, wB).Scan(&required, &confirmedAt); err != nil {
		t.Fatal(err)
	}
	if !required || confirmedAt != nil {
		t.Fatalf("new mission plan_confirm_required=%v plan_confirmed_at=%v, want true/nil", required, confirmedAt)
	}

	out := f.post(t, map[string]any{"content": router.MentionLink("Lead", f.leadUUID) + " 시작", "work_id": wB})
	leadTask := mustUUID(t, str(out["triggers"].([]any)[0].(map[string]any), "task_id"))
	f.runTask(t, leadTask)

	// (gate) delegating before the plan is confirmed is refused — nothing
	// handed to a teammate yet.
	if _, err := f.srv.Router.Delegate(ctx, leadTask, testCard(f.rUUID, "조사")); problemCode(err) != "plan_confirmation_required" {
		t.Fatalf("(gate) err = %v, want plan_confirmation_required", err)
	}

	var laneID uuid.UUID
	if err := f.pool.QueryRow(ctx, `SELECT lane_id FROM task WHERE id = $1`, leadTask).Scan(&laneID); err != nil {
		t.Fatal(err)
	}
	tok, err := f.srv.Tokens.Issue(ctx, f.pool, tokens.Scope{
		TaskID: leadTask, Attempt: currentAttempt(t, f, leadTask), LaneID: laneID, SessionID: mustUUID(t, f.sessionID), AgentID: f.leadUUID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// (approval-only) plan_confirm on a question is refused — it is a
	// confusing no-op, not a silently ignored flag.
	f.hitlOn(t, tok, map[string]any{
		"type": "question", "question": "계획이 맞나요?", "proposed_default": "네", "plan_confirm": true,
	}, 422)

	reg := f.hitlOn(t, tok, map[string]any{
		"type": "approval", "summary": "미션 계획을 확인해 주세요",
		"context": "목표: 조사 2건 위임. 접근법: Researcher에게 시장 조사, Writer에게 초안. 범위: 이번 주 안에 완료.",
		"plan_confirm": true,
	}, 201)
	hitlID := str(reg["hitl_request"].(map[string]any), "id")
	if got := str(reg["hitl_request"].(map[string]any), "purpose"); got != "plan" {
		t.Fatalf("stored purpose = %q, want plan", got)
	}
	f.endTurn(t, leadTask)

	// Still blocked while the Director has not answered.
	if err := f.pool.QueryRow(ctx, `SELECT plan_confirmed_at FROM work WHERE id = $1`, wB).Scan(&confirmedAt); err != nil {
		t.Fatal(err)
	}
	if confirmedAt != nil {
		t.Fatalf("plan_confirmed_at = %v, want nil before the Director answers", confirmedAt)
	}

	// The Director confirms the plan.
	f.api.must(200, "POST", f.p+"/hitl-requests/"+hitlID+"/response",
		map[string]any{"approved": true}, "Idempotency-Key", uuid.NewString())
	if err := f.pool.QueryRow(ctx, `SELECT plan_confirmed_at FROM work WHERE id = $1`, wB).Scan(&confirmedAt); err != nil {
		t.Fatal(err)
	}
	if confirmedAt == nil {
		t.Fatal("(gate-lifts) plan_confirmed_at = nil after an approved purpose: plan HITL — the gate never lifts")
	}

	// (gate-lifts) the answer requeued a new attempt on the SAME task — now
	// delegation succeeds.
	f.runTask(t, leadTask)
	res, err := f.srv.Router.Delegate(ctx, leadTask, testCard(f.rUUID, "조사"))
	if err != nil {
		t.Fatalf("(gate-lifts) delegate after confirmation = %v, want success", err)
	}
	if res.Card.Assignee.AgentId != f.rUUID {
		t.Errorf("delegated to %s, want %s", res.Card.Assignee.AgentId, f.rUUID)
	}
}
