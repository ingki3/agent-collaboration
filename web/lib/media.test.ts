/**
 * lib/media.ts 의 순수 함수(그 중에서도 아티팩트 열기 — 실사용 요청 2026-10-14). 미디어 판정·크기 표기는
 * `lib/mock/media.test.ts`(목이 서버와 같은 규칙인지)가 이미 잰다 — 여기는 `openUrl`·`isMarkdown`만.
 */
import { describe, expect, it } from "vitest";
import { isMarkdown, openUrl } from "./media";

describe("openUrl — /artifacts/:id, 새 탭에서 여는 자리", () => {
  it("id 를 그대로 경로에 쓰고, 특수문자는 인코딩한다", () => {
    expect(openUrl("a1")).toBe("/artifacts/a1");
    expect(openUrl("a/b")).toBe("/artifacts/a%2Fb");
  });
});

describe("isMarkdown — text/markdown 만(파라미터·대소문자 무시)", () => {
  it("마크다운이면 true, 그 밖은(없거나 다른 타입) false", () => {
    expect(isMarkdown("text/markdown")).toBe(true);
    expect(isMarkdown("text/markdown; charset=utf-8")).toBe(true);
    expect(isMarkdown("TEXT/MARKDOWN")).toBe(true);
    expect(isMarkdown("text/plain")).toBe(false);
    expect(isMarkdown("application/pdf")).toBe(false);
    expect(isMarkdown(null)).toBe(false);
    expect(isMarkdown(undefined)).toBe(false);
  });
});
