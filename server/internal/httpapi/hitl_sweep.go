package httpapi

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ingki3/agent-collabortion/server/internal/hitl"
	"github.com/ingki3/agent-collabortion/server/internal/inbox"
	"github.com/ingki3/agent-collabortion/server/internal/router"
	"github.com/ingki3/agent-collabortion/server/internal/tasks"
)

// SweepHitlDeadlines is FR-5.4's "기한 만료 시 동작", run by the scheduler.
//
// The rule is a grid of TYPE × autonomy and the type decides first: a
// `question` under `autonomous` proceeds with the agent's proposal, while an
// `approval` or an `info` request ALWAYS keeps waiting, in every autonomy.
// There is no auto-approve and no auto-reject — approving empties the human
// gate the request exists to be, and rejecting kills healthy work (M7, E7-14,
// E7-21).
//
// `expired` is not a status: an overdue request stays `open` and carries a
// flag, so a late answer is still an answer (E7-15).
//
// Returns how many requests it touched.
func (s *Server) SweepHitlDeadlines(ctx context.Context) (int, error) {
	now := s.Clock.Now()
	type due struct {
		id       uuid.UUID
		session  uuid.UUID
		task     *uuid.UUID
		kind     string
		autonomy string
		def      string
		question string
		work     *uuid.UUID
	}
	rows, err := s.DB.Query(ctx, `
		SELECT h.id, h.session_id, h.task_id, h.type::text, COALESCE(wk.autonomy, s.autonomy)::text,
		       -- T-S-wt: an isolation question (the repository a worktree room
		       -- splits from) is never auto-answered — without a default the
		       -- choice waits like an approval does (PlanExpiry).
		       CASE WHEN h.purpose = 'isolation' THEN '' ELSE COALESCE(h.proposed_default, '') END, h.question, wk.id
		FROM hitl_request h JOIN room s ON s.id = h.session_id
		LEFT JOIN task t ON t.id = h.task_id
		-- The request's mission decides its autonomy (T-R1b2): a mission may
		-- run under another autonomy than its room.
		LEFT JOIN work wk ON wk.id = COALESCE(h.work_id, t.work_id)
		WHERE h.status = 'open' AND h.due_at <= $1
		ORDER BY h.due_at`, now)
	if err != nil {
		return 0, err
	}
	var list []due
	for rows.Next() {
		var d due
		if err := rows.Scan(&d.id, &d.session, &d.task, &d.kind, &d.autonomy, &d.def, &d.question, &d.work); err != nil {
			rows.Close()
			return 0, err
		}
		list = append(list, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	n := 0
	for _, d := range list {
		p := hitl.PlanExpiry(d.kind, d.autonomy, d.def)
		err := s.inSessionTx(ctx, func(tx pgx.Tx) error {
			var status string
			if err := tx.QueryRow(ctx, `SELECT status::text FROM hitl_request WHERE id = $1 FOR UPDATE`, d.id).Scan(&status); err != nil {
				return err
			}
			if status != hitl.StatusOpen {
				return nil // answered between the scan and the lock
			}
			if p.Status == hitl.StatusOpen {
				// Overdue is a FLAG, and the inbox sorts it to the top.
				_, err := tx.Exec(ctx, `UPDATE hitl_request SET overdue = true WHERE id = $1 AND NOT overdue`, d.id)
				return err
			}
			if _, err := tx.Exec(ctx, `
				UPDATE hitl_request SET status = 'auto_answered', answer = $2, answered_at = $3
				WHERE id = $1`, d.id, p.Answer, now); err != nil {
				return err
			}
			// E7-12: the decision is marked automatic. "The Director said 투자자"
			// and "nobody answered and we used the agent's proposal" read
			// identically in the log without it.
			if _, err := insertDecision(ctx, tx, d.session,
				d.question+" → "+p.Answer, "기한 만료로 에이전트 제안대로 진행", "hitl", &d.id, true, now, d.work); err != nil {
				return err
			}
			if err := inbox.Resolve(ctx, s.Hub, tx, d.id, now); err != nil {
				return err
			}
			// T-APPROVAL: 「자동 응답됨」 reaches the open card live.
			s.publishHitlVia(ctx, tx, uuid.Nil, d.session, d.id, "hitl.updated")
			return nil
		})
		if err != nil {
			return n, err
		}
		if p.TaskStatus == "queued" && d.task != nil {
			if _, err := s.Tasks.ResumeFromHuman(ctx, *d.task, tasks.CauseHitlAnswer); err != nil {
				return n, err
			}
			s.Queue.Notifier.Notify()
		}
		n++
	}
	return n, nil
}

// SweepDeputyPings is FR-2A.1b 2단계(v0.19.31)'s active ping — `SweepHitlDeadlines`'s
// sibling. `hitl.Authorize` computes "기한 절반 경과" only when something reads
// it (a GET), so the moment a mission's deputy actually becomes able to
// answer a `director`-only request passes with nothing happening unless
// someone happens to reopen it (실측: 「게임 기획」 방 13시간 방치). This sweep
// catches that moment once per request: it resurfaces the deputy's existing
// hitl_request card — they may have read it before they could act on it,
// which would otherwise hide it from their badge for good — and names them
// in the room so the moment is visible without anyone reloading an inbox.
//
// Scoped to approver_spec = 'director' requests whose mission has a deputy
// (FR-2A.1b 1단계 requires one on every mission opened after that shipped):
// SpecRoomOwner's own half-deadline hand-over is a different approver chain
// (roomgate.LoadApprovers) with its own banner (SCREEN §4.5), not a
// per-request ping.
//
// Returns how many requests it pinged.
func (s *Server) SweepDeputyPings(ctx context.Context) (int, error) {
	now := s.Clock.Now()
	type due struct {
		id, session, wsID, deputy uuid.UUID
		question                  string
	}
	rows, err := s.DB.Query(ctx, `
		SELECT h.id, h.session_id, s.workspace_id, wk.deputy_user_id, h.question
		FROM hitl_request h JOIN room s ON s.id = h.session_id
		LEFT JOIN task t ON t.id = h.task_id
		JOIN work wk ON wk.id = COALESCE(h.work_id, t.work_id)
		WHERE h.status = 'open' AND h.approver_spec = 'director' AND h.deputy_notified_at IS NULL
		  AND wk.deputy_user_id IS NOT NULL
		  AND h.created_at + (h.due_at - h.created_at) / 2 <= $1`, now)
	if err != nil {
		return 0, err
	}
	var list []due
	for rows.Next() {
		var d due
		if err := rows.Scan(&d.id, &d.session, &d.wsID, &d.deputy, &d.question); err != nil {
			rows.Close()
			return 0, err
		}
		list = append(list, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	n := 0
	for _, d := range list {
		err := s.inSessionTx(ctx, func(tx pgx.Tx) error {
			var status string
			var notified *time.Time
			if err := tx.QueryRow(ctx, `SELECT status::text, deputy_notified_at FROM hitl_request WHERE id = $1 FOR UPDATE`, d.id).
				Scan(&status, &notified); err != nil {
				return err
			}
			if status != hitl.StatusOpen || notified != nil {
				return nil // answered, or already pinged, between the scan and the lock
			}
			// The card has been there since creation (hitlInbox files both
			// director and deputy); only read_at can be hiding it now.
			if _, err := tx.Exec(ctx, `
				UPDATE inbox_item i SET read_at = NULL
				FROM member m WHERE m.id = i.member_id AND m.workspace_id = $3 AND m.user_id = $2
				  AND i.ref_id = $1 AND i.type = 'hitl_request'`, d.id, d.deputy, d.wsID); err != nil {
				return err
			}
			inbox.NotifySummary(ctx, s.Hub, tx, d.wsID, d.deputy, now)
			name := displayName(ctx, tx, d.deputy)
			if _, err := s.Router.SystemPost(ctx, tx, d.session,
				router.UserMentionLink(name, d.deputy)+" 님, 기한의 절반이 지나 이제 이 요청에 답할 수 있습니다: 「"+firstLineOf(d.question)+"」"); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `UPDATE hitl_request SET deputy_notified_at = $2 WHERE id = $1`, d.id, now)
			return err
		})
		if err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

var _ = time.Now
