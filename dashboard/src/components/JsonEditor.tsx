import { useState } from "react";

interface Props {
  value: Record<string, unknown>;
  onChange: (v: Record<string, unknown>) => void;
  placeholder?: string;
}

/**
 * A freeform JSON editor that lets users type raw JSON.
 * Shows a parse error inline but doesn't block input.
 */
export function JsonEditor({ value, onChange, placeholder }: Props) {
  const [raw, setRaw] = useState(() => JSON.stringify(value, null, 2));
  const [error, setError] = useState("");

  function handleChange(text: string) {
    setRaw(text);
    try {
      const parsed = JSON.parse(text);
      if (typeof parsed === "object" && parsed !== null && !Array.isArray(parsed)) {
        onChange(parsed);
        setError("");
      } else {
        setError("Must be a JSON object { }");
      }
    } catch {
      setError("Invalid JSON");
    }
  }

  return (
    <div>
      <textarea
        rows={8}
        value={raw}
        onChange={(e) => handleChange(e.target.value)}
        placeholder={placeholder ?? '{\n  "key": "value"\n}'}
        spellCheck={false}
        className="w-full rounded-md border bg-background px-3 py-2 font-mono text-xs outline-none focus:ring-2 focus:ring-ring resize-y"
      />
      {error && <p className="mt-1 text-xs text-destructive">{error}</p>}
    </div>
  );
}
