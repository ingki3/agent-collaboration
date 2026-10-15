import { describe, expect, it } from "vitest";
import { DEFAULT_AVATARS, isSameAvatar } from "./avatars";

describe("DEFAULT_AVATARS — 30종, 서로 다른 주소", () => {
  it("30개, 모두 /avatars/*.svg, 중복 없음", () => {
    expect(DEFAULT_AVATARS).toHaveLength(30);
    for (const a of DEFAULT_AVATARS) expect(a.url).toMatch(/^\/avatars\/avatar-\d{2}\.svg$/);
    expect(new Set(DEFAULT_AVATARS.map((a) => a.url)).size).toBe(30);
    expect(new Set(DEFAULT_AVATARS.map((a) => a.id)).size).toBe(30);
  });
});

describe("isSameAvatar — 공백 차이만 다르면 같은 선택으로 본다", () => {
  it("null·undefined·빈 문자열은 서로 같다(모두 '선택 없음')", () => {
    expect(isSameAvatar(null, undefined)).toBe(true);
    expect(isSameAvatar(undefined, "")).toBe(true);
  });
  it("앞뒤 공백 차이는 무시", () => {
    expect(isSameAvatar(" /avatars/avatar-01.svg", "/avatars/avatar-01.svg ")).toBe(true);
  });
  it("다른 주소는 다르다", () => {
    expect(isSameAvatar("/avatars/avatar-01.svg", "/avatars/avatar-02.svg")).toBe(false);
  });
});
