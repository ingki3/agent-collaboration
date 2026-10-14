"use client";
/**
 * HITL Card(COMPONENTS §2.3 `n9PqY`) — **S7 타임라인 자리**의 겉껍질. 본문은 `HitlBody` 로 빠져 있어
 * Inbox Item(S8)과 같은 코드를 쓴다(리뷰 #03 K2, PLAN §8 결정 6).
 *
 * 겉껍질이 담는 것: kind 배지(`HITL · approval` / `HITL · question`) · 작성자(에이전트 or "시스템") ·
 * 메타(시스템 발행이면 **"… 때문에 발행 · source: system"**) · 상태(open / answered / auto_answered /
 * cancelled "취소됨").
 *
 * **`purpose` 가 시스템 발행의 판정 기준이다**(계약 `HitlRequest.purpose`, 0012·PR #103·#108) —
 * `source: system` + `type: approval` 만으로는 완료 승인과 예산·루프 정지를 구분할 수 없다.
 */
import "./hitl-card.css";
import { HitlBody, HITL_STATUS_LABEL, type HitlAction } from "./HitlBody";
import { clockTime } from "@/lib/time";
import { WORK_PANEL } from "@/lib/wording";
import type { HitlRequest, HitlResponse } from "@/lib/api/types";

type Purpose = NonNullable<HitlRequest["purpose"]>;

/**
 * 시스템 발행 사유 문구(purpose 별). 에이전트 발행(`agent`)에는 이 줄이 없다 — 발행 이유가 곧 질문이기 때문이다.
 * 문자열은 계약 enum 값(`user_approval`·`budget`·`time`·`loop`)에 붙어 있다.
 */
export const PURPOSE_REASON: Record<Exclude<Purpose, "agent">, string> = {
  user_approval: "종료 조건(Director 승인)",
  budget: "예산 상한 초과",
  time: "시간 상한 도달",
  loop: "주고받기 상한 도달",
  // v0.2.2 계약 — 첫 실행 격리 확인(FR-2.1.1). 화면 반영은 R2.
  isolation: "첫 실행 격리 확인",
};

/** 메타 한 줄(COMPONENTS §2.3 `i2BcH`). 시스템 발행이면 사유 + `source: system`. */
export function hitlMetaLine(r: Pick<HitlRequest, "source" | "purpose" | "created_at">): string {
  if (r.source !== "system") return `${clockTime(r.created_at)} · 에이전트가 요청`;
  const p = r.purpose && r.purpose !== "agent" ? PURPOSE_REASON[r.purpose] : null;
  return p ? `${p} 때문에 자동으로 올라온 요청` : "시스템이 올린 요청";
}

/** 카드 머리 라벨 — `question`·`approval` 은 계약 enum 이고 화면에는 사람의 말로 선다(§8.4). */
export function hitlKindLabel(r: Pick<HitlRequest, "type">): string {
  return r.type === "approval" ? "사람 확인 · 승인" : "사람 확인 · 질문";
}

export interface HitlCardProps {
  request: HitlRequest;
  /** 낼 응답 컨트롤. 비우면 타입 기본값(question → 답변, approval → 승인·거절). */
  actions?: HitlAction[];
  onRespond?: (body: HitlResponse) => Promise<void> | void;
  /**
   * 예산 초과 시스템 HITL 이면 승인에 상향 입력을 붙인다(E9-02).
   * 비워 두면 `purpose` 로만 판정한다 — 범위(`scope`)·현재 상한·소진액을 아는 것은 호출부다.
   */
  budget?: { scope?: "task" | "session"; current?: number | null; spent?: number | null } | null;
  busy?: boolean;
  /** `answered_by`(사용자 id) → 이름. 답한 뒤 카드가 「누가·언제·무엇」 을 말한다(T-APPROVAL). */
  userName?: (userId: string) => string | null | undefined;
  /**
   * 완료 승인(`purpose: user_approval`) 카드의 요약 줄("작업 카드 N장 — …") 아래에 분담표로 가는 링크를
   * 붙인다(실사용 요청 2026-10-14 — 숫자만으로는 무슨 카드가 왜 그런지 알 수 없었다). `HitlRequest` 에는
   * 미션 id 가 없어(계약에 없음) 그 요청의 메시지가 아는 `work_id` 를 호출부(`TimelineItemView`)가 넘긴다.
   */
  workId?: string | null;
  onOpenWorkBoard?: (workId: string) => void;
}

/**
 * 머리줄(배지·누가 요청했는지·언제 왜) — **타임라인 카드와 인박스 항목이 같은 모양을 쓴다**(실사용 요청
 * 2026-10-14: 인박스의 `hitl_request` 항목에 요청자가 안 보였다). 인박스는 `HitlRequest` 전체를 들고
 * 있지 않으므로(`InboxItem.card` 가 더 얇다) 이 둘만 받는다 — `HitlCard` 는 `r`에서 뽑아 넘긴다.
 */
export function HitlCardHead({ type, status, author, meta }: { type: HitlRequest["type"]; status: HitlRequest["status"]; author: string; meta: string }) {
  return (
    <div className="hitl__head">
      <span className="hitl__kind" data-testid="hitl-kind">
        <span aria-hidden="true">⏳︎</span>
        {hitlKindLabel({ type })}
      </span>
      <span className="hitl__author" data-testid="hitl-author">{author}</span>
      <span className="hitl__meta" data-testid="hitl-meta">{meta}</span>
      {status !== "open" && (
        <span className="hitl__meta" data-testid="hitl-status-label">{HITL_STATUS_LABEL[status]}</span>
      )}
    </div>
  );
}

export function HitlCard({ request: r, actions, onRespond, budget, busy, userName, workId, onOpenWorkBoard }: HitlCardProps) {
  const author = r.source === "system" ? "시스템" : (r.agent?.name ?? "에이전트");
  // 완료 승인 카드의 카드 요약 줄("작업 카드 N장 — …")이 `context` 에 실려 있을 때만 보인다 — 다른 HITL(질문·
  // 예산·루프·격리)은 그 줄이 없고, 카드가 0장인 미션의 완료 승인도 `context` 가 비어 있다(서버 cards.Summary).
  const boardLink = r.purpose === "user_approval" && r.context && workId && onOpenWorkBoard;
  return (
    <article
      className={`hitl${r.status === "open" ? " hitl--open" : ""}${r.status === "cancelled" ? " hitl--cancelled" : ""}`}
      data-testid="hitl-card"
      data-hitl-id={r.id}
      data-source={r.source}
      data-purpose={r.purpose ?? ""}
      data-status={r.status}
    >
      <HitlCardHead type={r.type} status={r.status} author={author} meta={hitlMetaLine(r)} />
      <HitlBody
        type={r.type}
        status={r.status}
        question={r.question}
        context={r.context}
        options={r.options}
        proposedDefault={r.proposed_default}
        dueAt={r.due_at}
        overdue={r.overdue}
        canRespond={r.can_respond}
        canRespondFrom={r.can_respond_from}
        actions={actions}
        answer={r.answer}
        approved={r.approved}
        answeredBy={r.answered_by ? (userName?.(r.answered_by) ?? null) : null}
        answeredAt={r.answered_at}
        // 조건은 `purpose` 하나다 — 세션이 멈췄는지는 보지 않는다(task 범위 초과는 세션을 멈추지 않는다, W-6).
        budgetOverride={
          r.purpose === "budget"
            ? { scope: r.task_id ? "task" : "session", current: r.budget_override_usd, ...(budget ?? {}) }
            : null
        }
        onRespond={onRespond}
        busy={busy}
      />
      {boardLink && (
        <button type="button" className="hitl__board-link" onClick={() => onOpenWorkBoard(workId)} data-testid="hitl-open-board">
          {WORK_PANEL.board_from_approval}
        </button>
      )}
    </article>
  );
}

export default HitlCard;
