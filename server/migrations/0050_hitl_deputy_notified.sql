-- 0050_hitl_deputy_notified.sql — deputy 능동 ping (FR-2A.1b 2단계, v0.19.31, Director 결정 2026-10-17)
--
-- "기한 절반 경과 후 deputy가 답할 수 있다"는 지금까지 읽는 시점에만 계산되는 값
-- (hitl.Authorize)이라, 그 경계를 실제로 넘는 순간 deputy에게 아무 일도 안 일어난다.
-- 분 단위 스케줄러가 그 순간을 감지해 한 번만 능동 알림(받은 요청 갱신 + 방 시스템
-- 메시지)을 보내도록, "이미 알렸는가"를 이 칸에 못 박는다.

ALTER TABLE hitl_request ADD COLUMN deputy_notified_at timestamptz;
