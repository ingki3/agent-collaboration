-- 0049_work_plan_confirm.sql — 미션 킥오프 계획 확인 (선택, 실사용 요청 2026-10-14)
--
-- Director 가 미션을 열 때 선택적으로 켜면, 위임(card_delegate)이 처음 걸릴 때까지
-- 서버가 그걸 막는다 — Lead 는 이해한 목표·접근법·범위를 approval HITL(purpose:
-- plan)로 올려 Director 승인을 먼저 받아야 한다. 기본은 꺼짐(기존 미션 전부 그대로).

ALTER TABLE work ADD COLUMN plan_confirm_required boolean NOT NULL DEFAULT false;
ALTER TABLE work ADD COLUMN plan_confirmed_at timestamptz;

ALTER TABLE hitl_request DROP CONSTRAINT hitl_request_purpose_ck;
ALTER TABLE hitl_request ADD CONSTRAINT hitl_request_purpose_ck
    CHECK (purpose IS NULL OR purpose IN ('agent', 'user_approval', 'budget', 'time', 'loop', 'isolation', 'plan'));
