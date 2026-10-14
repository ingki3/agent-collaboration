"use client";
/**
 * 아티팩트 열기(`/artifacts/:id`) — 방을 열지 않고도 산출물을 바로 확인할 수 있게 하는 자리(실사용 요청
 * 2026-10-14: "url을 통해 브라우저로 열 수 있도록, 마크다운인 경우 html로 렌더링").
 *
 * `RoomPanel` 의 아티팩트 줄·결과 카드의 근거 칩이 이름을 이 경로로 링크한다(새 탭).
 *   · `text/markdown` 이면 본문을 받아 `lib/markdown.tsx` 의 `<Markdown>` 으로 그 자리에서 HTML 렌더링한다 —
 *     메시지 본문이 이미 쓰는 같은 렌더러라 서식이 타임라인과 같다.
 *   · 이미지·영상·소리(FR-4.3.1 미리보기 목록)는 `MediaGroup` 으로 인라인 재생.
 *   · 그 밖은 내려받기 링크만 — 서버가 `?inline=true` 를 무시하고 `attachment` 로 주는 타입이라 화면이
 *     대신 열어 줄 방법이 없다(브라우저 자체 PDF 뷰어 등은 `target="_blank"` 로도 일부 되지만 보장하지 않는다).
 */
import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import "./artifact-page.css";
import { Markdown } from "@/lib/markdown";
import { MediaGroup } from "@/components/MediaPreview";
import { api, errorMessage } from "@/lib/api/client";
import { downloadUrl, fetchArtifactText, formatBytes, isMarkdown, mediaKind } from "@/lib/media";
import { relativeTime } from "@/lib/time";
import type { Artifact } from "@/lib/api/types";

export default function ArtifactPage() {
  const { id } = useParams<{ id: string }>();
  const [artifact, setArtifact] = useState<Artifact | null>(null);
  const [text, setText] = useState<string | null>(null);
  const [textError, setTextError] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setArtifact(null);
    setText(null);
    setTextError(null);
    setError(null);
    void api
      .get("/artifacts/{artifactId}", { path: { artifactId: id } })
      .then((a) => {
        if (cancelled) return;
        setArtifact(a);
        // 메타는 이미 받았다 — 본문을 못 읽어도(권한·삭제·연결) 이름·버전·내려받기 링크는 그대로 둔다.
        // 아티팩트 자체를 못 읽을 때만(아래 catch) 평문 problem 줄로 통째로 바꾼다.
        if (!isMarkdown(a.content_type)) return;
        fetchArtifactText(id)
          .then((t) => { if (!cancelled) setText(t); })
          .catch((e) => { if (!cancelled) setTextError(errorMessage(e)); });
      })
      .catch((e) => { if (!cancelled) setError(errorMessage(e)); });
    return () => { cancelled = true; };
  }, [id]);

  if (error) return <div className="page" data-testid="artifact-page-error"><p className="problem" role="alert">{error}</p></div>;
  if (!artifact) return <div className="page" data-testid="artifact-page-loading">불러오는 중…</div>;

  const kind = mediaKind(artifact.content_type);
  const markdown = isMarkdown(artifact.content_type);

  return (
    <div className="page artifact-page" data-testid="artifact-page">
      <header className="artifact-page__head">
        <h1 data-testid="artifact-page-name">{artifact.name}</h1>
        <p className="artifact-page__meta">
          v{artifact.version} · {artifact.content_type ?? artifact.type} · {formatBytes(artifact.size_bytes)} ·{" "}
          {artifact.submitted_by?.agent_name ?? "—"} · {relativeTime(artifact.created_at)}
        </p>
        <a className="artifact-page__link" href={downloadUrl(id)} data-testid="artifact-page-download">
          원본 내려받기
        </a>
      </header>
      {markdown && text != null ? (
        <Markdown content={text} />
      ) : markdown && textError ? (
        <p className="muted" data-testid="artifact-page-md-error">{textError} — 위 링크로 내려받으세요.</p>
      ) : markdown ? (
        <p className="muted" data-testid="artifact-page-md-loading">읽는 중…</p>
      ) : kind !== "file" ? (
        <MediaGroup items={[artifact]} bare testId="artifact-page-media" />
      ) : (
        <p className="muted" data-testid="artifact-page-no-preview">
          이 형식은 화면에서 바로 보여 줄 수 없습니다 — 위 링크로 내려받으세요.
        </p>
      )}
    </div>
  );
}
