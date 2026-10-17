package sessions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ingki3/agent-collabortion/server/internal/apperr"
	"github.com/ingki3/agent-collabortion/server/internal/db"
	"github.com/ingki3/agent-collabortion/server/internal/quiet"
)

// T-APPROVAL — the platform's user_approval request waits for the mission's
// work to settle (PRD FR-2.3 완료 흐름, openapi v0.3.3 held_reason).

// EventTasksSettled is the ApplyEvent kind ReleaseHeldApproval uses: it
// satisfies no atom, it only re-reads the tree over the atoms already met —
// the same shape as EventConditionChanged — so a mission whose only missing
// atom is user_approval now gets the request it was held from.
const EventTasksSettled = "tasks_settled"

// HeldRunningTasks is the `held_reason` value (openapi v0.3.3).
const HeldRunningTasks = "running_tasks"

// busyTaskSQL is "a task of mission $1 is running or about to": the four
// statuses T-APPROVAL names. `deferred` (a fallback not yet due) and the
// parked ones (`paused`, `waiting_human`) do not hold the request — a
// question waiting on the Director must not wait on itself.
const busyTaskSQL = `EXISTS (SELECT 1 FROM task t WHERE t.work_id = $1
	AND t.status IN ('queued', 'dispatched', 'preparing', 'running')
	AND NOT ` + quiet.HeldTaskSQL + `)`

func missionBusy(ctx context.Context, q db.DBTX, workID uuid.UUID) (bool, error) {
	var busy bool
	if err := q.QueryRow(ctx, `SELECT `+busyTaskSQL, workID).Scan(&busy); err != nil {
		return false, fmt.Errorf("sessions: mission busy: %w", err)
	}
	return busy, nil
}

// revisionAddressed is T-REVISION's gate (complete.go, `approvalAsk &&
// out.HitlIssued`): whether the mission's MOST RECENT Director rejection of
// a `user_approval` request has a task_card accepted after it. No rejection
// yet → nothing to gate, true. An earlier rejection an accepted card already
// answered does not re-arm this — only the latest one counts, so a second
// rejection on fresh grounds needs its own new card.
//
// `task_card.updated_at` catches either shape work took in practice: a
// brand-new numbered card, or an existing one revised in place and
// re-accepted (version bumped, same number) — both are the agent doing
// something, not just restating the old artifact.
func revisionAddressed(ctx context.Context, q db.DBTX, workID uuid.UUID) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx, `
		SELECT NOT EXISTS (
			SELECT 1 FROM hitl_request h
			WHERE h.work_id = $1 AND h.purpose = $2 AND h.status = 'answered' AND h.approved = false
		) OR EXISTS (
			SELECT 1 FROM task_card c
			WHERE c.work_id = $1 AND c.status = 'accepted' AND c.updated_at > (
				SELECT h.answered_at FROM hitl_request h
				WHERE h.work_id = $1 AND h.purpose = $2 AND h.status = 'answered' AND h.approved = false
				ORDER BY h.answered_at DESC LIMIT 1
			)
		)`, workID, CondUserApproval).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("sessions: revision addressed: %w", err)
	}
	return ok, nil
}

// ReleaseHeldApproval re-reads a held mission once its work may have
// settled. It is cheap when nothing is held (one indexed read) so the task
// layer calls it after every task that ends, whatever the ending — finish,
// cancel or failure.
//
// production callers: tasks.Service.AfterSettle (wired in httpapi.NewServer)
// and ReleaseHeldApprovals (the scheduler's catch-all).
func (s *Service) ReleaseHeldApproval(ctx context.Context, workID uuid.UUID) (bool, error) {
	var due bool
	err := s.DB.QueryRow(ctx, `
		SELECT approval_held_at IS NOT NULL AND status = 'active' AND NOT `+busyTaskSQL+`
		FROM work WHERE id = $1`, workID).Scan(&due)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !due) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("sessions: held approval: %w", err)
	}
	out, err := s.ApplyWorkEvent(ctx, workID, Event{Kind: EventTasksSettled})
	if err != nil {
		var p *apperr.Problem
		if errors.As(err, &p) && p.Status == 409 {
			return false, nil // closed between the read and the lock
		}
		return false, err
	}
	return out.HitlIssued, nil
}

// ReleaseHeldApprovals is the scheduler's catch-all for the paths that end a
// task inside someone else's transaction (a room block, the kill switch):
// every held mission whose work has settled gets its request.
func (s *Service) ReleaseHeldApprovals(ctx context.Context) (int, error) {
	rows, err := s.DB.Query(ctx, `SELECT id FROM work WHERE approval_held_at IS NOT NULL AND status = 'active'`)
	if err != nil {
		return 0, err
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	n := 0
	for _, id := range ids {
		opened, err := s.ReleaseHeldApproval(ctx, id)
		if err != nil {
			return n, err
		}
		if opened {
			n++
		}
	}
	return n, nil
}

// ReleaseStalledRevisions is T-REVISION's own catch-all, the companion to
// ReleaseHeldApprovals: AfterSettle only fires for the WORK_ID of the task
// that just ended, so a mission whose completion approval was rejected and
// then only settled through follow-up work with NO mission selected
// ("일 밖 task", work_id NULL) never gets its own hook called again — it can
// sit `active` forever even once the revision is genuinely done.
//
// Real incident (실사용 「금융 에이전트 시장 규모 추정 방」, 2026-10-14): the
// Director rejected completion, the actual revision happened as follow-up
// chat work outside the mission, and Lead separately asked its own
// agent-sourced approval HITL ("최종 보고서 ... 승인 요청") that LOOKED like
// the mission-closing question. The Director approved that, both sides
// believed the mission was done, but `work.status` stayed `active` and no
// second formal `user_approval` request was ever raised.
//
// Scoped tightly — only missions with an answered, rejected user_approval
// request in their history and no currently open one are candidates; a
// mission that was never rejected never matches and this is a no-op for it.
// revisionAddressed (inside ApplyWorkEvent) still decides whether a fresh
// request actually opens, so this never re-asks over an unaddressed
// rejection — it only gives a STUCK mission another chance to re-evaluate.
func (s *Service) ReleaseStalledRevisions(ctx context.Context) (int, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT w.id FROM work w
		WHERE w.status = 'active'
		  AND NOT EXISTS (SELECT 1 FROM hitl_request h WHERE h.work_id = w.id AND h.source = 'system'
		                    AND h.purpose = $1 AND h.status = 'open')
		  AND EXISTS (SELECT 1 FROM hitl_request h WHERE h.work_id = w.id
		                AND h.purpose = $1 AND h.status = 'answered' AND h.approved = false)`,
		CondUserApproval)
	if err != nil {
		return 0, fmt.Errorf("sessions: stalled revisions: %w", err)
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	n := 0
	for _, id := range ids {
		out, err := s.ApplyWorkEvent(ctx, id, Event{Kind: EventTasksSettled})
		if err != nil {
			var p *apperr.Problem
			if errors.As(err, &p) && p.Status == 409 {
				continue // closed between the read and the lock
			}
			return n, err
		}
		if out.HitlIssued {
			n++
		}
	}
	return n, nil
}

// SettleQuiet is T-QUIET's re-entry (b) (Lead 판정 2026-09-28): a mission a
// person re-opened (quiet.StateReleased) goes quiet again once its work has
// settled while the platform's approval request is still open — the round
// of work the person asked for is over and the card is still the question.
// Cheap when nothing applies (one indexed read), so the task layer calls it
// after every task that ends, beside ReleaseHeldApproval.
func (s *Service) SettleQuiet(ctx context.Context, workID uuid.UUID) (bool, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	var due bool
	err = tx.QueryRow(ctx, `
		SELECT approval_quiet = 'released' AND status = 'active' AND NOT `+busyTaskSQL+`
		   AND EXISTS (SELECT 1 FROM hitl_request h WHERE h.work_id = $1 AND h.source = 'system'
		                 AND h.purpose = 'user_approval' AND h.status = 'open')
		FROM work WHERE id = $1`, workID).Scan(&due)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !due) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("sessions: settle quiet: %w", err)
	}
	// The room row first — the lock order of every writer of both (router,
	// ApplyWorkEvent).
	if _, err := tx.Exec(ctx, `SELECT 1 FROM room WHERE id = (SELECT room_id FROM work WHERE id = $1) FOR UPDATE`, workID); err != nil {
		return false, err
	}
	changed, err := quiet.Enter(ctx, tx, workID, s.Clock.Now())
	if err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return changed, nil
}

// NotifyMissionIdle is FR-2A.2.1b (Director 결정 2026-10-13): the safety net
// for a mission whose completion tree never gets re-read because nobody ever
// fires a qualifying event (artifact submit, review, condition change). Real
// incident (「1인 창업 사업 계획서 작성」 방, 2026-10-13): every lane settled,
// cards reported, but nobody called `colab artifact submit`, so
// ApplyWorkEvent never ran once and the mission's user_approval request
// (its only completion atom) sat unraised forever.
//
// This does NOT touch completion state — it only READS the tree (Satisfied /
// needsUserApproval, same pure functions ApplyWorkEvent uses) to word the
// notice, then posts a system message and a `work_idle` inbox item. It never
// writes completion_met, never opens a hitl_request, and is a pure no-op
// while the formal approval flow (FR-2A.2.1~.3) is already handling things:
// held (`approval_held_at`) or an open `user_approval` request both skip it.
//
// Cheap when nothing applies (one indexed-ish read), so the task layer calls
// it after every task that ends, beside ReleaseHeldApproval and SettleQuiet.
func (s *Service) NotifyMissionIdle(ctx context.Context, workID uuid.UUID) (bool, error) {
	var wsID, sessionID, director uuid.UUID
	var raw, metRaw []byte
	var due bool
	err := s.DB.QueryRow(ctx, `
		SELECT w.room_id, r.workspace_id, w.director_user_id, w.completion_condition, w.completion_met,
		       w.status = 'active' AND w.approval_held_at IS NULL AND NOT `+busyTaskSQL+`
		           AND NOT EXISTS (SELECT 1 FROM hitl_request h WHERE h.work_id = w.id AND h.source = 'system'
		                             AND h.purpose = 'user_approval' AND h.status = 'open')
		           AND NOT EXISTS (SELECT 1 FROM inbox_item i WHERE i.work_id = w.id AND i.type = 'work_idle' AND i.read_at IS NULL)
		FROM work w JOIN room r ON r.id = w.room_id WHERE w.id = $1`,
		workID).Scan(&sessionID, &wsID, &director, &raw, &metRaw, &due)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !due) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("sessions: mission idle: %w", err)
	}

	met := map[string]bool{}
	_ = json.Unmarshal(metRaw, &met)
	// Satisfied/needsUserApproval only read Condition.Type, never .Agent, so
	// the assignee resolution ApplyWorkEvent does before evaluating is not
	// needed here — this is wording only, never a namesActor() call.
	tree := ParseTree(raw)
	body := "위임된 작업이 모두 끝나 미션이 조용해졌습니다.\n"
	switch {
	case Satisfied(tree, met):
		// Vanishingly rare here (Satisfied without user_approval would have
		// completed the mission already through the normal path) but kept
		// honest rather than asserting it cannot happen.
		body += "완료 조건은 이미 다 찼습니다."
	case needsUserApproval(tree, met):
		body += "완료 조건은 승인만 남았습니다 — 끝났다고 보시면 미션을 끝내 주세요."
	default:
		body += "완료 조건은 아직 안 찼습니다(예: 아티팩트 제출) — 다음 행동을 정해 주세요."
	}

	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if s.Router != nil {
		if _, err := s.Router.SystemPostWork(ctx, tx, sessionID, &workID, body); err != nil {
			return false, fmt.Errorf("sessions: mission idle notice: %w", err)
		}
	}
	if err := WorkInboxItem(ctx, s.Hub, tx, wsID, director, TypeWorkIdle, sessionID, workID, s.Clock.Now()); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
