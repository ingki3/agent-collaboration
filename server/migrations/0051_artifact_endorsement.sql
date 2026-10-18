-- 0051_artifact_endorsement.sql — FR-2A.1a 채택(endorse, v0.19.28, Director 결정 2026-10-16)
--
-- `artifact_submitted(agent: X)` 은 지금까지 X 본인의 제출만 봤다 — X 가
-- 위임만 하는 역할(Lead 가 흔하다)이면 다른 에이전트가 산출물을 전부
-- 내도 그 원자는 영영 안 찬다. X 가 이미 제출된 아티팩트를 "내 제출로
-- 받아들인다"고 확인하는 것도 같은 종류의 사건(artifact_submit)으로
-- 재평가하되, 누가·언제·무엇을 보고 채택했는지는 따로 남긴다 — 리뷰
-- (agent_approval)와는 다른 원자·다른 역할이라 artifact_review 테이블을
-- 같이 쓰지 않는다.

CREATE TABLE artifact_endorsement (
    id                   bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    artifact_id          uuid NOT NULL REFERENCES artifact(id) ON DELETE CASCADE,
    endorsed_by_agent_id uuid NOT NULL REFERENCES agent(id),
    endorsed_by_task_id  uuid REFERENCES task(id) ON DELETE SET NULL,
    comment              text,
    decision_id          uuid REFERENCES decision(id) ON DELETE SET NULL,
    endorsed_at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX artifact_endorsement_artifact ON artifact_endorsement (artifact_id, endorsed_at DESC, id DESC);

-- artifact_review_latest(0024)와 같은 모양 — openapi Artifact.endorsement 가 읽는 자리.
CREATE VIEW artifact_endorsement_latest AS
    SELECT DISTINCT ON (artifact_id) *
    FROM artifact_endorsement
    ORDER BY artifact_id, endorsed_at DESC, id DESC;

COMMENT ON TABLE artifact_endorsement IS
  'FR-2A.1a(v0.19.28) 제출 담당의 채택 — append-only, artifact_review(agent_approval)와는 다른 원자.';
COMMENT ON VIEW artifact_endorsement_latest IS
  '아티팩트당 마지막 채택 한 행 — openapi Artifact.endorsement 가 읽는 자리.';
