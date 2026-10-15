/**
 * 기본 카툰 아바타(실사용 요청 2026-10-15: "에이전트별로 프로필 사진을 지정 ... 다양하게") — 30종, `public/avatars/*.svg`.
 * 피부색 7 · 머리 스타일 10여 종 · 안경·헤드폰·안테나·리본·꽃 등 장신구로 조합해 서로 다른 인상을 만든다
 * (안테나·헤드폰은 "에이전트"라는 맥락에 어울리는 장난기 있는 변주). 전부 로컬 SVG라 네트워크 의존이 없다.
 */
export interface DefaultAvatar {
  id: string;
  url: string;
}

export const DEFAULT_AVATARS: readonly DefaultAvatar[] = Array.from({ length: 30 }, (_, i) => {
  const n = String(i + 1).padStart(2, "0");
  return { id: `avatar-${n}`, url: `/avatars/avatar-${n}.svg` };
});

/** 두 아바타가 "같은 선택"인지 — 공백·대소문자 차이는 무시한다. */
export function isSameAvatar(a: string | null | undefined, b: string | null | undefined): boolean {
  return (a ?? "").trim() === (b ?? "").trim();
}
