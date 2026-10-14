/**
 * RoomPanel(우열 「방 전체」 칸) — 아티팩트 줄만 잰다(그 밖은 page.test.tsx 의 비용 테스트가 덮는다).
 *
 * 실사용 요청(2026-10-14): 아티팩트 이름이 그냥 글자였다 — 눌러서 열 길이 없었다. 이름을
 * `/artifacts/:id`(새 탭)로 링크해 브라우저에서 바로 열 수 있게 한다.
 */
import "@testing-library/jest-dom/vitest";
import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { RoomPanel } from "./RoomPanel";
import type { Artifact, Room } from "@/lib/api/types";

const room: Room = {
  id: "r1", workspace_id: "ws1", name: "사업 계획서 작성방", description: "", status: "active", visibility: "workspace",
  owner_user_id: "u1", deputy_owner_user_id: null, runtime_id: null, isolation: { kind: "none", remote_url: null },
  limits: { budget_usd: null, time_limit: null, max_concurrent_works: 3, max_parallel_lanes: 5 }, autonomy: "guided", default_director_user_id: null,
  blocked_reason: null, blocked_detail: null, counts: { works_active: 0, lanes_active: 0, tasks_active: 0 }, cost_usd: 0, cost_estimated: false,
  unread_count: 0, my_room_role: "owner", my_capabilities: ["post"],
  created_by: "u1", created_at: "", updated_at: "", last_activity_at: null,
};

const artifact = (over: Partial<Artifact> = {}): Artifact => ({
  id: "art1", session_id: "r1", name: "business-plan.md", version: 2, type: "report",
  storage_ref: "", size_bytes: 28753, content_type: "text/markdown; charset=utf-8",
  submitted_by_task_id: "t1", submitted_by: { agent_id: "a1", agent_name: "Lead" },
  description: null, latest: true, created_at: "2026-10-14T12:05:28Z", work_id: null,
  ...over,
});

describe("아티팩트 줄 — url 로 브라우저에서 연다(실사용 요청)", () => {
  it("이름이 /artifacts/:id 로 가는 링크다(새 탭) — 전엔 글자일 뿐이었다", () => {
    render(
      <RoomPanel
        room={room} works={[]} artifacts={[artifact()]} decisions={[]}
        selectedWorkId={null} open onToggle={() => undefined}
      />,
    );
    const link = screen.getByTestId("artifact-open");
    expect(link).toHaveTextContent("business-plan.md");
    expect(link).toHaveAttribute("href", "/artifacts/art1");
    expect(link).toHaveAttribute("target", "_blank");
  });
});
