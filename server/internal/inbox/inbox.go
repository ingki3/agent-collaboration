// Package inbox is FR-8 / SCREEN §4.6: what lands in a person's inbox, how
// urgent it is, and what they can do about it without opening the session.
//
// The severity table is a function rather than a literal at each insert site
// because the badge is derived from it (`getInboxSummary` counts
// action_required only) and a single miscategorised insert makes the badge
// either useless or alarming.
package inbox

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ingki3/agent-collaboration/server/internal/db"
	"github.com/ingki3/agent-collaboration/server/internal/realtime"
)

// NotifySummary pushes one person's fresh action_required/overdue/unread
// counts over SSE (`inbox.summary`, PublishTo — targeted, not broadcast: the
// counts are per-person). Same query GetInboxSummary answers on a GET,
// scoped here to the one user an insert/resolve/read-mark just touched.
//
// v0.19.31 2단계(2026-10-17 실사용 관찰): the contract has carried this event
// type since D1 and the web mock emits it, but the real server never
// published it — the badge only ever moved on reload/resync. Every writer of
// inbox_item (insert, ResolveByRefSQL, mark-read, roomgate's own inserts/
// resolves) calls this for whoever it just touched so the badge moves live.
// hub may be nil (tests without a Hub wired) — a no-op then, like every
// other optional-SSE call site in this codebase.
func NotifySummary(ctx context.Context, hub *realtime.Hub, q db.DBTX, ws, user uuid.UUID, now time.Time) {
	if hub == nil {
		return
	}
	var actionRequired, overdue, unread int
	err := q.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE i.severity = 'action_required'),
		       count(*) FILTER (WHERE h.id IS NOT NULL AND h.status = 'open' AND h.due_at < $3),
		       count(*) FILTER (WHERE i.read_at IS NULL)
		FROM inbox_item i
		JOIN member m ON m.id = i.member_id
		LEFT JOIN hitl_request h ON h.id = i.ref_id AND i.type = 'hitl_request'
		WHERE m.user_id = $1 AND m.workspace_id = $2`, user, ws, now).Scan(&actionRequired, &overdue, &unread)
	if err != nil {
		slog.Warn("inbox: summary frame", "err", err, "user", user)
		return
	}
	_ = hub.PublishTo(ctx, q, ws, nil, &user, "inbox.summary",
		summaryPayload{ActionRequired: actionRequired, Overdue: overdue, Unread: &unread})
}

// summaryPayload mirrors gen.InboxSummary's wire shape without importing
// internal/httpapi/gen — that package already imports internal/inbox for the
// type/severity/action tables, so the reverse import would cycle.
type summaryPayload struct {
	ActionRequired int  `json:"action_required"`
	Overdue        int  `json:"overdue"`
	Unread         *int `json:"unread,omitempty"`
}

// The seven item types (inbox_item_type, FR-8).
const (
	TypeHitlRequest    = "hitl_request"
	TypeLaneBlocked    = "lane_blocked"
	TypeRunFailed      = "run_failed"
	TypeRuntimeOffline = "runtime_offline"
	TypeMention        = "mention"
	// TypeWorkdirGCBlocked is FR-6.4's "삭제하지 않고 Director 에게 알린다"
	// (E13-12·13). Added in P4 by Lead decision (T-S9 ask 1); the contract's
	// InboxItemType grows the same value.
	TypeWorkdirGCBlocked = "workdir_gc_blocked"
	// TypeRoomPaused is the room gate (PRD v0.19 FR-2.4 · FR-8): budget or
	// loop stopped the WHOLE room and its owner is asked to let it go on.
	// ref_id is that request.
	TypeRoomPaused = "room_paused"
	// TypeIsolationConfirm is FR-2.1.1: the room's first run would land on a
	// computer with a repository while isolation is `none`. ref_id is the
	// room-owner approval (approve = worktree, reject = none).
	TypeIsolationConfirm = "isolation_confirm"

	// The rest of v0.2.0 (PRD v0.19, openapi InboxItemType). room_invited ·
	// workdir_quota are written by the room API (T-R1b3); work_* are the
	// mission stream's to write — their severity and actions are fixed here
	// so every writer lands on the same badge.
	TypeRoomInvited   = "room_invited"
	TypeWorkdirQuota  = "workdir_quota"
	TypeWorkProposed  = "work_proposed"
	TypeWorkPaused    = "work_paused"
	TypeWorkCompleted = "work_completed"
	// TypeWorkIdle is FR-2A.2.1b (Director 결정 2026-10-13): a mission whose
	// open lanes hit zero without ever raising a completion approval request
	// (no artifact submit/review/condition-change event ever fired one) gets
	// this safety-net notice instead of sitting silently forever. ref_id is
	// the system message that explains the mission's current state.
	TypeWorkIdle = "work_idle"
)

// Recipient bases (InboxItem.recipient_basis) — the card's 「Director 로서」·
// 「방장으로서」: why this item is in THIS person's inbox.
const (
	BasisDirector       = "director"
	BasisDeputy         = "deputy"
	BasisRoomOwner      = "room_owner"
	BasisRoomDeputy     = "room_deputy"
	BasisWorkspaceOwner = "workspace_owner"
)

// The three severities (inbox_severity, SCREEN §4.6).
const (
	ActionRequired = "action_required"
	Attention      = "attention"
	Info           = "info"
)

// Severity is SCREEN §4.6's table. The split is "does this WAIT for me?":
//
//	action_required  nothing moves until a person acts — an open HITL request,
//	                 a lane blocked on a question, a room or mission paused for
//	                 budget or a loop.
//	attention        something went wrong and a person should look, but the
//	                 platform is not holding a turn for them.
//	info             it happened; read it when you read the inbox.
//
// `info` items are deliberately outside the nav badge: counting them makes the
// badge a permanent number nobody reads.
func Severity(itemType string) string {
	switch itemType {
	case TypeHitlRequest, TypeLaneBlocked,
		TypeRoomPaused, TypeWorkPaused, TypeIsolationConfirm:
		return ActionRequired
	case TypeRunFailed, TypeRuntimeOffline, TypeWorkdirGCBlocked, TypeWorkdirQuota, TypeWorkProposed, TypeWorkIdle:
		return Attention
	case TypeMention, TypeRoomInvited, TypeWorkCompleted:
		return Info
	}
	return Info
}

// ResolveByRefSQL is every hitl_request-backed resolution's second write (the
// first answers the request itself): it takes the item out of action_required
// — a request that has been decided no longer gates anything — and, like the
// read_at-only write it replaces, marks it read so it also drops off the
// unread count. Params: $1 ref_id (the hitl_request id), $2 now.
//
// Scoped to hitl_request-backed types (hitl_request · room_paused ·
// isolation_confirm share ref_id = hitl_request.id): every write site used to
// set only read_at, leaving severity 'action_required' forever — an answered
// request stayed in the nav badge and the "조치 필요" filter indefinitely
// even though the request itself was long decided (실사용 2026-10-16).
const ResolveByRefSQL = `
	UPDATE inbox_item SET read_at = COALESCE(read_at, $2),
	       severity = CASE WHEN severity = 'action_required' THEN 'info'::inbox_severity ELSE severity END
	WHERE ref_id = $1`

// Resolve runs ResolveByRefSQL and notifies everyone it just touched —
// director and deputy both have a card for the same hitl_request (O5), so
// one answer moves two badges.
func Resolve(ctx context.Context, hub *realtime.Hub, q db.DBTX, refID uuid.UUID, now time.Time) error {
	rows, err := q.Query(ctx, `
		WITH updated AS (
		  UPDATE inbox_item SET read_at = COALESCE(read_at, $2),
		         severity = CASE WHEN severity = 'action_required' THEN 'info'::inbox_severity ELSE severity END
		  WHERE ref_id = $1
		  RETURNING member_id)
		SELECT m.workspace_id, m.user_id FROM updated u JOIN member m ON m.id = u.member_id`, refID, now)
	if err != nil {
		return err
	}
	type target struct{ Ws, User uuid.UUID }
	targets, err := pgx.CollectRows(rows, pgx.RowToStructByPos[target])
	if err != nil {
		return err
	}
	for _, t := range targets {
		NotifySummary(ctx, hub, q, t.Ws, t.User, now)
	}
	return nil
}

// OfflineRoomActions is a room_paused card for a lost computer (FR-9.2 v0.19):
// there is no request to approve — the way out is rebinding (the other one,
// cancelling every open mission, is on the room screen).
func OfflineRoomActions(canRespond bool) []string {
	if canRespond {
		return []string{"rebind", "open_room"}
	}
	return []string{"open_room"}
}

// Actions is the inline action list (openapi InboxItem.actions). It is
// permission-aware: an action the caller cannot take is not offered, because a
// button that 403s is worse than no button (FR-5.3 last bullet).
func Actions(itemType, hitlType string, canRespond bool) []string {
	switch itemType {
	case TypeHitlRequest:
		if !canRespond {
			return []string{"open_session"}
		}
		switch hitlType {
		case "approval":
			return []string{"approve", "reject", "open_session"}
		default:
			return []string{"answer", "open_session"}
		}
	// Room-owner approvals (T-R1b1) on a room-level card (openapi 0.2.1): the
	// room, not a mission, is what opens.
	case TypeIsolationConfirm:
		if canRespond {
			return []string{"approve", "reject", "open_room"}
		}
		return []string{"open_room"}
	case TypeRoomPaused:
		if canRespond {
			return []string{"approve_continue", "open_room"}
		}
		return []string{"open_room"}
	case TypeLaneBlocked, TypeMention:
		return []string{"reply", "open_session"}
	case TypeRunFailed:
		if canRespond {
			return []string{"restart", "open_session"}
		}
		return []string{"open_session"}
	case TypeWorkdirGCBlocked:
		// The two ways out FR-6.4 names. "정리" is the manual delete S13
		// offers once the person has merged or discarded.
		return []string{"open_workdirs", "delete_workdir"}
	case TypeRuntimeOffline:
		return []string{"open_runtimes"}
	case TypeWorkdirQuota:
		// FR-6.4: the way out is clearing folders on S13.
		return []string{"open_workdirs"}
	case TypeRoomInvited, TypeWorkProposed:
		// A room-level card (openapi 0.2.1): the room, not a mission, is what
		// opens. A proposal is not a mission yet.
		return []string{"open_room"}
	case TypeWorkPaused, TypeWorkCompleted, TypeWorkIdle:
		return []string{"open_work"}
	}
	return []string{"open_session"}
}

// SortRank is the group order of SCREEN §4.6: overdue first, then by severity.
// Within a group the caller orders by due_at, soonest first — an inbox sorted
// by arrival buries the thing that expires in ten minutes under five notices.
func SortRank(severity string, overdue bool) int {
	if overdue {
		return 0
	}
	switch severity {
	case ActionRequired:
		return 1
	case Attention:
		return 2
	}
	return 3
}
