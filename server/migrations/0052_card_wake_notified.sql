-- 0052_card_wake_notified.sql — 카드 wake 중복 통지 방지 (실사용 e2e 72_scenario_a 재현, 2026-11-07)
--
-- wakeDelegatorForResult 가 idle 위임자를 결과 카드로 깨운 뒤, 그 자식(들)의 lane 이
-- 곧이어 공식적으로 끝나면 maybeFireJoin 이 "같은 라운드"인지를 activeTaskFor 의
-- 순간 스냅숏(지금 당장 queued 인가)으로만 판단했다 — 실제로는 위임자의 그 깨움이
-- (실기와 마찬가지로 빠른 페이크 런타임에서도) 이미 claim 되어 completed 로
-- 끝나버린 뒤에 lane 이 끝나는 경우가 흔해서, 스냅숏이 "queued 아님"으로 보여
-- 같은 묶음을 두 번째 hop 으로 다시 깨웠다(FR-3.5 비용 낭비). 스냅숏이 아니라
-- 영속적인 표식이 있어야 "이 묶음은 이미 카드로 알렸다"를 나중에도 알 수 있다.

ALTER TABLE task ADD COLUMN cards_notified_at timestamptz;
