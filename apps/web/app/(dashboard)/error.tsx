"use client";

export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <div className="flex flex-1 items-center justify-center p-8">
      <div className="max-w-lg text-center">
        <h2 className="text-lg font-semibold text-destructive mb-2">
          Dashboard Error
        </h2>
        <p className="text-sm text-muted-foreground mb-4">
          {error.message || "Unknown error occurred"}
        </p>
        {error.digest && (
          <p className="text-xs text-muted-foreground mb-4">
            Digest: {error.digest}
          </p>
        )}
        <pre className="text-xs text-left bg-muted p-4 rounded-md overflow-auto max-h-64 mb-4">
          {error.stack}
        </pre>
        <button
          onClick={reset}
          className="px-4 py-2 bg-primary text-primary-foreground rounded-md"
        >
          Try again
        </button>
      </div>
    </div>
  );
}
