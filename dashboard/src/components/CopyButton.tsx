import { useState } from "react";

export function CopyButton({ value, label }: { value: string; label?: string }) {
  const [copied, setCopied] = useState(false);
  function copy() {
    navigator.clipboard.writeText(value);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }
  return (
    <button onClick={copy} className="rounded px-2 py-0.5 text-xs border hover:bg-muted transition-colors shrink-0">
      {copied ? "Copied!" : label ?? "Copy"}
    </button>
  );
}
