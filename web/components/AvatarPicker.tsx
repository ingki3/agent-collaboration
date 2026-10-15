"use client";
/**
 * 에이전트 아바타 선택(실사용 요청 2026-10-15) — 기본 카툰 아바타 30종(`lib/avatars.ts`) 중 고르거나,
 * 이미지 주소를 직접 입력한다. `agent.avatar_url` 은 서버·계약에 이미 있던 칸이라(AgentCreate·AgentUpdate)
 * 여기서는 화면만 새로 붙인다 — 마이그레이션도 서버 변경도 필요 없다.
 */
import { useState } from "react";
import "./avatar-picker.css";
import { DEFAULT_AVATARS, isSameAvatar } from "@/lib/avatars";

export interface AvatarPickerProps {
  value: string | null;
  onChange: (url: string | null) => void;
  disabled?: boolean;
  /** 아바타가 없을 때 동그라미 안에 보일 이니셜(이름 첫 글자). */
  fallbackLetter: string;
}

export function AvatarPicker({ value, onChange, disabled, fallbackLetter }: AvatarPickerProps) {
  const [open, setOpen] = useState(false);
  const [customUrl, setCustomUrl] = useState(value ?? "");

  return (
    <div className="avatar-picker" data-testid="avatar-picker">
      <div className="avatar-picker__row">
        <span className="avatar-picker__preview" data-testid="avatar-preview">
          {value ? <img src={value} alt="" /> : <span aria-hidden="true">{fallbackLetter}</span>}
        </span>
        <div className="avatar-picker__actions">
          <button
            type="button" className="btn btn--sm" disabled={disabled}
            aria-expanded={open} onClick={() => { setCustomUrl(value ?? ""); setOpen((o) => !o); }}
            data-testid="avatar-picker-toggle"
          >
            {open ? "닫기" : "사진 바꾸기"}
          </button>
          {value && (
            <button
              type="button" className="btn btn--sm btn--ghost" disabled={disabled}
              onClick={() => onChange(null)} data-testid="avatar-clear"
            >
              이니셜로 되돌리기
            </button>
          )}
        </div>
      </div>
      {open && (
        <div className="avatar-picker__panel" data-testid="avatar-picker-panel">
          <p className="field__label" style={{ margin: "0 0 6px" }}>기본 아바타 (30종 중 고르기)</p>
          <div className="avatar-picker__grid" role="listbox" aria-label="기본 아바타 고르기">
            {DEFAULT_AVATARS.map((a) => (
              <button
                key={a.id} type="button" className="avatar-picker__opt"
                data-selected={isSameAvatar(value, a.url) ? "true" : undefined}
                aria-pressed={isSameAvatar(value, a.url)}
                disabled={disabled}
                onClick={() => { onChange(a.url); setCustomUrl(a.url); }}
                data-testid="avatar-option" data-avatar-id={a.id}
              >
                <img src={a.url} alt="" />
              </button>
            ))}
          </div>
          <label className="field" style={{ marginTop: 10 }}>
            <span className="field__label">또는 이미지 주소 직접 입력</span>
            <div className="row">
              <input
                className="input" type="url" placeholder="https://…" value={customUrl} disabled={disabled}
                onChange={(e) => setCustomUrl(e.target.value)} data-testid="avatar-url-input"
              />
              <button
                type="button" className="btn btn--sm" disabled={disabled || !customUrl.trim()}
                onClick={() => onChange(customUrl.trim())} data-testid="avatar-url-apply"
              >
                적용
              </button>
            </div>
          </label>
        </div>
      )}
    </div>
  );
}

export default AvatarPicker;
