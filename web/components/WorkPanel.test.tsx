/**
 * WorkPanel — 「개요」 탭의 제목 줄 중복 제거(실사용 리뷰 2026-10-14: 같은 긴 문장이 "최근 활동" 줄·굵은
 * 제목·"미션 목표" 셋 중 최대 세 자리에 보였다). 이 조건은 코드 리뷰에서 테스트가 하나도 없다고 지적됐다 —
 * 이 파일이 그 공백을 메운다. 다른 WorkPanel 동작(동작 버튼·조건 행 등)은 room 화면 통합 테스트가 커버한다.
 */
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { WorkPanel } from "./WorkPanel";
import type { Work } from "@/lib/api/types";

afterEach(cleanup);

const work = (over: Partial<Work> = {}): Work => ({
  id: "w1", room_id: "r1", title: "국내 금융 에이전트 시장 규모 추정", goal: "국내 금융 에이전트 시장 규모 추정",
  acceptance_criteria: [], director_user_id: "u1", director: { id: "u1", email: "d@x", display_name: "Director", avatar_url: null, created_at: "2026-10-14T00:00:00Z" },
  deputy_user_id: null, assignee_agent_id: "a1",
  completion_condition: { op: "and", conditions: [{ type: "artifact_submitted", who: "assignee" }, { type: "user_approval" }] },
  completion_progress: { met: 1, total: 2, satisfied: false, human_gate: true, conditions: [
    { path: "/conditions/0", type: "artifact_submitted", met: true, met_at: "2026-10-14T09:00:00Z", met_by: "a1", agent_id: "a1", agent_name: "Lead", next_actor: null, blocked_reason: null, hitl_request_id: null },
    { path: "/conditions/1", type: "user_approval", met: false, met_at: null, met_by: null, agent_id: null, agent_name: null, next_actor: "director", blocked_reason: null, hitl_request_id: null },
  ] },
  limits: { budget_usd: null, budget_tokens: null, time_limit: null, max_tasks: null },
  autonomy: "guided", status: "active", paused_reason: null, cost_usd: 16.42, cost_estimated: true,
  my_work_role: "director", created_by: "u1", created_at: "2026-10-14T09:00:00Z", updated_at: "2026-10-14T09:00:00Z",
  started_at: "2026-10-14T09:00:00Z", finished_at: null, last_activity_at: "2026-10-14T09:00:00Z",
  ...over,
});

describe("개요 탭 — 제목 줄 중복 제거", () => {
  it("picked 모드 · title ≠ goal — 굵은 제목 줄이 보인다", () => {
    render(<WorkPanel mode={{ kind: "picked", workId: "w1" }} work={work({ title: "시장 규모 추정", goal: "국내 금융 에이전트 시장 규모를 추정한다" })} />);
    expect(screen.getByTestId("work-title").textContent).toBe("시장 규모 추정");
    // 미션 목표 쪽은 그대로 전문을 보인다 — 중복 제거는 제목 줄에만 적용된다.
    expect(screen.getByTestId("work-goal").textContent).toBe("국내 금융 에이전트 시장 규모를 추정한다");
  });

  it("picked 모드 · title === goal(공백 차이만) — 굵은 제목 줄이 없다, 미션 목표는 그대로 보인다", () => {
    render(<WorkPanel mode={{ kind: "picked", workId: "w1" }} work={work({ title: " 같은 문장 ", goal: "같은 문장" })} />);
    expect(screen.queryByTestId("work-title")).toBeNull();
    expect(screen.getByTestId("work-goal").textContent).toBe("같은 문장");
  });

  it("recent 모드 — '최근 활동' 줄이 이미 제목을 말하므로, title 이 goal 과 달라도 굵은 제목 줄은 없다", () => {
    render(<WorkPanel mode={{ kind: "recent", workId: "w1" }} work={work({ title: "짧은 제목", goal: "완전히 다른 긴 목표 문장" })} />);
    expect(screen.getByTestId("work-panel-recent").textContent).toContain("짧은 제목");
    expect(screen.queryByTestId("work-title")).toBeNull();
    expect(screen.getByTestId("work-goal").textContent).toBe("완전히 다른 긴 목표 문장");
  });
});
