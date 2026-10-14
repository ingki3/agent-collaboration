/**
 * 아티팩트 열기(`/artifacts/:id`, 실사용 요청 2026-10-14) — url 로 브라우저에서 바로 연다.
 *
 * 못 박는 것:
 *   1. `text/markdown` 이면 본문을 받아 HTML 로 그린다(`<Markdown>`) — 서버에 묻지 않고 다운로드로 떨어지지 않는다.
 *   2. 이미지 등 미리보기 목록(FR-4.3.1)은 `MediaGroup` 으로.
 *   3. 그 밖(예: PDF)은 미리보기 없이 다운로드 링크만 — 지어내지 않는다.
 */
import "@testing-library/jest-dom/vitest";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import type { Artifact } from "@/lib/api/types";

vi.mock("next/navigation", () => ({ useParams: () => ({ id: "a1" }) }));

const get = vi.fn();
vi.mock("@/lib/api/client", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api/client")>("@/lib/api/client");
  return { ...actual, api: { ...actual.api, get: (...a: unknown[]) => get(...a) } };
});

import ArtifactPage from "./page";

afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

const artifact = (over: Partial<Artifact> = {}): Artifact => ({
  id: "a1", session_id: "r1", name: "business-plan.md", version: 2, type: "report",
  storage_ref: "", size_bytes: 28753, content_type: "text/markdown; charset=utf-8",
  submitted_by_task_id: "t1", submitted_by: { agent_id: "a1", agent_name: "Lead" },
  description: null, latest: true, created_at: "2026-10-14T12:05:28Z", work_id: null,
  ...over,
});

describe("마크다운 아티팩트 — 그 자리에서 HTML 로", () => {
  it("본문을 받아 Markdown 으로 그린다 — 다운로드로 떨어지지 않는다", async () => {
    get.mockResolvedValue(artifact());
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, text: () => Promise.resolve("# 사업계획서\n\n**요약**: 좋음") });
    vi.stubGlobal("fetch", fetchMock);
    render(<ArtifactPage />);
    expect(await screen.findByTestId("artifact-page-name")).toHaveTextContent("business-plan.md");
    await waitFor(() => expect(screen.getByRole("heading", { level: 1, name: "사업계획서" })).toBeInTheDocument());
    expect(screen.getByText("요약").tagName).toBe("STRONG");
    expect(fetchMock).toHaveBeenCalledWith("/api/v1/artifacts/a1/content", expect.objectContaining({ credentials: "same-origin" }));
    expect(screen.getByTestId("artifact-page-download")).toHaveAttribute("href", "/api/v1/artifacts/a1/content");
  });

  it("본문을 읽지 못해도(권한·삭제) 통째로 오류 화면이 되지 않는다 — 이름·다운로드 링크는 그대로", async () => {
    get.mockResolvedValue(artifact());
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: false, status: 404 }));
    render(<ArtifactPage />);
    expect(await screen.findByTestId("artifact-page-name")).toHaveTextContent("business-plan.md");
    await waitFor(() => expect(screen.getByTestId("artifact-page-md-error")).toHaveTextContent("아티팩트를 읽지 못했습니다 (404)"));
    expect(screen.getByTestId("artifact-page-download")).toHaveAttribute("href", "/api/v1/artifacts/a1/content");
    expect(screen.queryByTestId("artifact-page-error")).toBeNull();
  });
});

describe("미디어 아티팩트 — MediaGroup 으로 미리보기", () => {
  it("이미지는 MediaGroup 자리에 그려진다", async () => {
    get.mockResolvedValue(artifact({ name: "diagram.png", content_type: "image/png" }));
    render(<ArtifactPage />);
    await screen.findByTestId("artifact-page-name");
    expect(screen.getByTestId("artifact-page-media")).toBeInTheDocument();
    expect(get).toHaveBeenCalledWith("/artifacts/{artifactId}", { path: { artifactId: "a1" } });
  });
});

describe("그 밖의 형식 — 미리보기 없이 다운로드만", () => {
  it("지어내지 않는다 — 「화면에서 바로 보여 줄 수 없습니다」 + 다운로드 링크", async () => {
    get.mockResolvedValue(artifact({ name: "report.pdf", content_type: "application/pdf" }));
    render(<ArtifactPage />);
    await screen.findByTestId("artifact-page-name");
    expect(screen.getByTestId("artifact-page-no-preview")).toBeInTheDocument();
    expect(screen.getByTestId("artifact-page-download")).toHaveAttribute("href", "/api/v1/artifacts/a1/content");
  });
});

describe("오류", () => {
  it("아티팩트를 못 읽으면(404·401) 평문 대신 problem 줄", async () => {
    get.mockRejectedValue(Object.assign(new Error("찾을 수 없습니다"), { status: 404 }));
    render(<ArtifactPage />);
    expect(await screen.findByTestId("artifact-page-error")).toHaveTextContent("찾을 수 없습니다");
  });
});
