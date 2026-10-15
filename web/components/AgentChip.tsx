"use client";
import "./agent-chip.css";
import { Badge } from "./Badge";
import type { AgentStatus, AgentRole } from "@/lib/api/types";

/** PRD FR-1.3 파생 순서(SCREEN §4.5): disabled > offline > error > working > waiting_human > idle. */
export const AGENT_STATUS_ORDER: readonly AgentStatus[] = ["disabled", "offline", "error", "working", "waiting_human", "idle"];

export interface DeriveInput {
  disabled?: boolean;
  /** 런타임 오프라인(세션 런타임 기준). */
  offline?: boolean;
  /** 재시도 없는 실패(auth·quota·config)를 가진 task 가 있는가. */
  error?: boolean;
  /** 이 에이전트의 task 상태 목록. blocked·paused lane 은 파생에 들어가지 않는다(C1′). */
  taskStatuses?: readonly string[];
}

/**
 * 상태의 **주인은 서버**다 — 화면은 `Participant.status` 를 그대로 그린다. 이 함수는 실시간 `task.updated` 만 받아
 * 참여자 이벤트가 아직 안 온 순간의 **거울**이고, 서버 값을 덮어쓰지 않는다. 규칙은 한 곳(여기)에만 둔다.
 *
 * **`working` 은 `running` task 뿐이다**(PRD FR-1.3 4행, W-5). `dispatched`·`preparing` 은 아직 턴이 시작되지 않았고,
 * 그것을 `working` 으로 세면 데몬이 claim 만 하고 멈춰도 칩이 계속 "작업 중"이라 침묵과 실행을 구분할 수 없다.
 * `blocked`·`paused` lane 도 파생에 넣지 않는다(C1′) — 왜 멈췄는지는 lane 카드가 말한다.
 */
export function deriveAgentStatus(input: DeriveInput): AgentStatus {
  if (input.disabled) return "disabled";
  if (input.offline) return "offline";
  if (input.error) return "error";
  const ts = input.taskStatuses ?? [];
  if (ts.some((s) => s === "running")) return "working";
  if (ts.some((s) => s === "waiting_human")) return "waiting_human";
  return "idle";
}

export const AGENT_ROLE_LABEL: Record<AgentRole, string> = {
  lead: "lead",
  researcher: "researcher",
  writer: "writer",
  engineer: "engineer",
  reviewer: "reviewer",
  custom: "custom",
};

export interface AgentChipProps {
  /**
   * 이 칩이 가리키는 에이전트의 id. `data-agent-id` 로 나가고, **e2e 가 칩을
   * 하나로 특정하는 유일한 수단**이다 — 이름은 워크스페이스 안에서만 유일하고
   * 화면에는 `@` 가 붙은 채로 나오며, 시나리오가 같은 이름의 에이전트를 둘
   * 만들면 셀렉터가 조용히 둘을 잡는다(G4 2판 W7: 셀렉터가 아무것도 못 잡았다).
   * 데모 화면처럼 진짜 id 가 없는 자리에서는 생략한다.
   */
  agentId?: string;
  name: string;
  role?: AgentRole;
  /** 파생 상태값. `derive` 를 주면 그것으로 계산한다(둘 다 있으면 derive 우선). */
  status?: AgentStatus;
  derive?: DeriveInput;
  /** 둘째 줄 부가 텍스트 — 상태 값이 아니다("lane #2 질문 대기", N2). */
  statusNote?: string | null;
  /** 프로파일 요약(예 "Claude Code · sonnet"). 둘째 줄에 role 과 함께. */
  profile?: string | null;
  avatarUrl?: string | null;
  isAssignee?: boolean;
  /**
   * 보관된 에이전트(`archived_at`). **과거 세션의 칩은 그대로 두고 "보관됨"만 붙인다**
   * (SCREEN §8.2 Q3 Lead 결정) — 지난 대화에서 누가 말했는지가 사라지면 기록이 읽히지 않는다.
   * 신규 초대 목록에서 빼는 쪽은 `ParticipantsDialog` 다.
   */
  archived?: boolean;
  size?: "md" | "sm";
  onClick?: () => void;
  className?: string;
}

export function AgentChip(props: AgentChipProps) {
  const status = props.derive ? deriveAgentStatus(props.derive) : (props.status ?? "idle");
  const line2 = [props.role && AGENT_ROLE_LABEL[props.role], props.profile, props.statusNote].filter(Boolean).join(" · ");
  const Tag = props.onClick ? "button" : "div";
  const size = props.size ?? "md";
  return (
    <Tag
      type={props.onClick ? "button" : undefined}
      className={["agent-chip", size === "sm" ? "agent-chip--sm" : "", props.className].filter(Boolean).join(" ")}
      onClick={props.onClick}
      data-testid="agent-chip"
      data-agent-id={props.agentId}
      data-status={status}
      data-archived={props.archived ? "true" : undefined}
    >
      <span className="agent-chip__avatar" aria-hidden="true">
        {props.avatarUrl ? <img src={props.avatarUrl} alt="" /> : props.name.slice(0, 1).toUpperCase()}
      </span>
      <span className="agent-chip__text">
        <span className="agent-chip__line1">
          <span className="agent-chip__name">@{props.name}</span>
          {props.isAssignee && <span className="agent-chip__assignee">담당</span>}
          {props.archived && (
            <span className="agent-chip__archived" data-testid="agent-chip-archived">보관됨</span>
          )}
          <Badge kind="agent" value={status} size="sm" />
          {/* 상세 정보(역할·런타임·모델·부가 메모)는 더 이상 칩에 바로 안 보인다 — i 아이콘에 올리면 레이어로 뜬다
              (실사용 요청 2026-10-14: 참여자 목록이 너무 복잡하다). 레이어는 숨겨져 있을 뿐 DOM 에는 그대로 있다. */}
          {size === "md" && line2 && (
            // 접근성 이름을 트리거에 바로 붙인다(실사용 리뷰 2026-10-15) — 레이어는 포커스·호버 때만
            // 보이므로(CSS) 그 타이밍에 기대는 aria-describedby 대신, 포커스 즉시 읽히는 aria-label 로
            // 내용을 전부 실어 보낸다. 시각 레이어는 aria-hidden 으로 중복 낭독을 막는다.
            <span className="agent-chip__info" tabIndex={0} aria-label={`${props.name} 상세 정보: ${line2}`} data-testid="agent-chip-info">
              <span className="agent-chip__info-glyph" aria-hidden="true">ⓘ</span>
              <span className="agent-chip__info-pop" aria-hidden="true" data-testid="agent-chip-line2">
                {line2}
              </span>
            </span>
          )}
        </span>
      </span>
    </Tag>
  );
}

export default AgentChip;
