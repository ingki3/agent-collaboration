import { describe, expect, it } from "vitest";
import { isRoomScreenPath } from "./Shell";

describe("isRoomScreenPath — Room 화면(S7)에서만 내비를 아이콘 레일로 접는다", () => {
  it("방 화면(/rooms/:id)은 접힌다", () => {
    expect(isRoomScreenPath("/rooms/836f83b2-f893-4412-be79-86ce29ad508d")).toBe(true);
  });

  it("방 목록(/rooms)은 접히지 않는다", () => {
    expect(isRoomScreenPath("/rooms")).toBe(false);
  });

  it("방 만들기(/rooms/new)는 접히지 않는다 — 단순 다이얼로그다", () => {
    expect(isRoomScreenPath("/rooms/new")).toBe(false);
  });

  it("방 하위 페이지(설정·참여자·읽음 기록)는 접히지 않는다 — 단순 폼·표다", () => {
    expect(isRoomScreenPath("/rooms/r1/settings")).toBe(false);
    expect(isRoomScreenPath("/rooms/r1/participants")).toBe(false);
    expect(isRoomScreenPath("/rooms/r1/reads")).toBe(false);
  });

  it("방과 무관한 화면은 접히지 않는다", () => {
    expect(isRoomScreenPath("/inbox")).toBe(false);
    expect(isRoomScreenPath("/agents")).toBe(false);
  });
});
