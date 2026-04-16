"use client";

/**
 * MermaidBlock — renders a mermaid code block as an inline SVG diagram.
 *
 * - Lazy-loads mermaid.js on first render (dynamic import, AC3)
 * - Falls back to raw <pre><code> with an error hint when syntax is invalid (AC2)
 * - Shows a loading skeleton while the library loads
 */

import { useEffect, useId, useState } from "react";

interface MermaidBlockProps {
  code: string;
}

export function MermaidBlock({ code }: MermaidBlockProps) {
  // useId produces a stable ID like ":r0:" — strip colons, prefix for mermaid
  const rawId = useId();
  const id = `mermaid-${rawId.replace(/:/g, "")}`;

  const [svg, setSvg] = useState<string | null>(null);
  const [error, setError] = useState(false);

  useEffect(() => {
    let cancelled = false;

    import("mermaid")
      .then(({ default: mermaid }) => {
        mermaid.initialize({ startOnLoad: false, theme: "neutral" });
        return mermaid.render(id, code);
      })
      .then(({ svg: rendered }) => {
        if (!cancelled) setSvg(rendered);
      })
      .catch(() => {
        if (!cancelled) setError(true);
      });

    return () => {
      cancelled = true;
    };
  }, [code, id]);

  if (error) {
    // AC2 — invalid syntax fallback
    return (
      <div className="mermaid-error">
        <pre>
          <code>{code}</code>
        </pre>
        <span className="mermaid-error-hint">Invalid diagram syntax</span>
      </div>
    );
  }

  if (!svg) {
    // AC3 — loading skeleton while mermaid.js bundle loads
    return <div className="mermaid-loading" aria-busy="true" />;
  }

  // AC1 — render the SVG inline
  return (
    <div
      className="mermaid-diagram"
      // mermaid sanitizes its own SVG output; no user-supplied HTML passthrough
      dangerouslySetInnerHTML={{ __html: svg }}
    />
  );
}
