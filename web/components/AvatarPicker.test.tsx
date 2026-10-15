/**
 * AvatarPicker(실사용 요청 2026-10-15) — 기본 카툰 아바타 30종 중 고르거나 주소를 직접 입력, "이니셜로 되돌리기".
 */
import { describe, expect, it, afterEach, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { AvatarPicker } from "./AvatarPicker";
import { DEFAULT_AVATARS } from "@/lib/avatars";

afterEach(cleanup);

describe("미리보기 — 아바타 없으면 이니셜, 있으면 이미지", () => {
  it("값이 없으면 이니셜 글자", () => {
    render(<AvatarPicker value={null} onChange={vi.fn()} fallbackLetter="L" />);
    expect(screen.getByTestId("avatar-preview").textContent).toBe("L");
    expect(screen.queryByTestId("avatar-clear")).toBeNull();
  });

  it("값이 있으면 이미지 + '이니셜로 되돌리기' 버튼", () => {
    render(<AvatarPicker value="/avatars/avatar-01.svg" onChange={vi.fn()} fallbackLetter="L" />);
    expect(screen.getByTestId("avatar-preview").querySelector("img")?.getAttribute("src")).toBe("/avatars/avatar-01.svg");
    expect(screen.queryByTestId("avatar-clear")).not.toBeNull();
  });
});

describe("기본 아바타 30종 — 고르면 그 주소로 onChange", () => {
  it("패널을 열면 30개 보이고, 하나 누르면 그 url 로 콜백", () => {
    const onChange = vi.fn();
    render(<AvatarPicker value={null} onChange={onChange} fallbackLetter="R" />);
    expect(screen.queryByTestId("avatar-picker-panel")).toBeNull();
    fireEvent.click(screen.getByTestId("avatar-picker-toggle"));
    const opts = screen.getAllByTestId("avatar-option");
    expect(opts).toHaveLength(DEFAULT_AVATARS.length);
    fireEvent.click(opts[4]);
    expect(onChange).toHaveBeenCalledWith(DEFAULT_AVATARS[4].url);
  });

  it("현재 값과 같은 기본 아바타만 선택 표시(aria-pressed)", () => {
    render(<AvatarPicker value={DEFAULT_AVATARS[2].url} onChange={vi.fn()} fallbackLetter="R" />);
    fireEvent.click(screen.getByTestId("avatar-picker-toggle"));
    const opts = screen.getAllByTestId("avatar-option");
    expect(opts[2].getAttribute("aria-pressed")).toBe("true");
    expect(opts[0].getAttribute("aria-pressed")).toBe("false");
    expect(opts[2].getAttribute("data-selected")).toBe("true");
  });
});

describe("직접 주소 입력", () => {
  it("입력 후 적용을 누르면 그 값으로 onChange, 빈 값이면 적용 비활성", () => {
    const onChange = vi.fn();
    render(<AvatarPicker value={null} onChange={onChange} fallbackLetter="W" />);
    fireEvent.click(screen.getByTestId("avatar-picker-toggle"));
    expect((screen.getByTestId("avatar-url-apply") as HTMLButtonElement).disabled).toBe(true);
    fireEvent.change(screen.getByTestId("avatar-url-input"), { target: { value: "https://example.com/me.png" } });
    expect((screen.getByTestId("avatar-url-apply") as HTMLButtonElement).disabled).toBe(false);
    fireEvent.click(screen.getByTestId("avatar-url-apply"));
    expect(onChange).toHaveBeenCalledWith("https://example.com/me.png");
  });
});

describe("이니셜로 되돌리기", () => {
  it("누르면 onChange(null)", () => {
    const onChange = vi.fn();
    render(<AvatarPicker value="/avatars/avatar-01.svg" onChange={onChange} fallbackLetter="W" />);
    fireEvent.click(screen.getByTestId("avatar-clear"));
    expect(onChange).toHaveBeenCalledWith(null);
  });
});

describe("disabled — 아무 조작도 못 한다", () => {
  it("토글·지우기 버튼이 비활성", () => {
    render(<AvatarPicker value="/avatars/avatar-01.svg" onChange={vi.fn()} fallbackLetter="W" disabled />);
    expect((screen.getByTestId("avatar-picker-toggle") as HTMLButtonElement).disabled).toBe(true);
    expect((screen.getByTestId("avatar-clear") as HTMLButtonElement).disabled).toBe(true);
  });
});
