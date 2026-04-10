import { useState, useRef, useEffect } from "react";

// All autocomplete suggestions for sentinel expressions
const SUGGESTIONS = [
  // ── Shorthands — most common, shown first ──────────────────────────────────
  { insert: "$authenticated", detail: "true if user is logged in — the most common gate", category: "shorthand" },
  { insert: "$unauthenticated", detail: "true if no auth token was sent", category: "shorthand" },
  { insert: "$admin", detail: "true if user has the 'admin' role", category: "shorthand" },
  { insert: "$owner", detail: "true if $req.user.id == $doc.owner_id", category: "shorthand" },
  { insert: "$verified", detail: "true if user's email is verified", category: "shorthand" },
  // ── User fields ────────────────────────────────────────────────────────────
  { insert: "$req.user.id", detail: "Authenticated user's ID", category: "user" },
  { insert: "$req.user.email", detail: "Authenticated user's email", category: "user" },
  { insert: "$req.user.roles", detail: "User's roles array — use with contains", category: "user" },
  { insert: "$req.user.verified", detail: "Whether user's email is verified", category: "user" },
  { insert: "$req.user.plan", detail: "Custom field from user data (example)", category: "user" },
  { insert: "$req.user.team_id", detail: "Custom field from user data (example)", category: "user" },
  { insert: "$req.user.data.", detail: "Any field from user.Data map", category: "user" },
  // ── Document fields ────────────────────────────────────────────────────────
  { insert: "$doc.owner_id", detail: "Document owner field (example)", category: "doc" },
  { insert: "$doc.published", detail: "Document published field (example)", category: "doc" },
  { insert: "$doc.status", detail: "Document status field (example)", category: "doc" },
  { insert: "$doc.team_id", detail: "Document team_id field (example)", category: "doc" },
  { insert: "$doc.", detail: "Any document field", category: "doc" },
  // ── Request context ────────────────────────────────────────────────────────
  { insert: "$req.ip", detail: "Client IP address", category: "req" },
  { insert: "$req.method", detail: "HTTP method (GET, POST, PATCH, DELETE)", category: "req" },
  // ── Operators ──────────────────────────────────────────────────────────────
  { insert: "==", detail: "Equal", category: "op" },
  { insert: "!=", detail: "Not equal", category: "op" },
  { insert: "contains", detail: "String/array contains: $req.user.roles contains \"admin\"", category: "op" },
  { insert: "startswith", detail: "String starts with: $req.user.email startswith \"admin\"", category: "op" },
  { insert: "endswith", detail: "String ends with: $req.user.email endswith \"@acme.com\"", category: "op" },
  { insert: "in", detail: "Value in array: $req.user.id in $doc.editors", category: "op" },
  { insert: "matches", detail: "Glob match (* wildcard): $req.user.email matches \"*@acme.com\"", category: "op" },
  { insert: "exists", detail: "Field is not null/empty: exists $doc.approved_at", category: "op" },
  { insert: "&&", detail: "Logical AND", category: "op" },
  { insert: "||", detail: "Logical OR", category: "op" },
  { insert: "!", detail: "Logical NOT", category: "op" },
];

const CATEGORY_COLORS: Record<string, string> = {
  shorthand: "bg-purple-100 text-purple-700",
  user: "bg-blue-100 text-blue-700",
  doc: "bg-green-100 text-green-700",
  req: "bg-orange-100 text-orange-700",
  op: "bg-muted text-muted-foreground",
};

interface SentinelInputProps {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  label?: string;
  desc?: string;
}

export function SentinelInput({ value, onChange, placeholder, label, desc }: SentinelInputProps) {
  const [open, setOpen] = useState(false);
  const [activeIdx, setActiveIdx] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  // Get the current "word" being typed (last token in the expression)
  function currentToken(v: string) {
    // Split by spaces and operators to find what the user is typing
    const match = v.match(/(\$[\w.]*|[\w]+)$/);
    return match ? match[1] : "";
  }

  const token = currentToken(value);

  const filtered = token.length === 0
    ? SUGGESTIONS.slice(0, 8) // show top 8 when nothing typed
    : SUGGESTIONS.filter((s) =>
        s.insert.toLowerCase().includes(token.toLowerCase()) ||
        s.detail.toLowerCase().includes(token.toLowerCase())
      ).slice(0, 10);

  useEffect(() => {
    setActiveIdx(0);
  }, [token]);

  function applySuggestion(s: typeof SUGGESTIONS[0]) {
    const tok = currentToken(value);
    const base = tok.length > 0 ? value.slice(0, value.length - tok.length) : value;
    // Add a trailing space after operators so the user can keep typing
    const trail = s.category === "doc" || s.category === "user" ? "" : " ";
    onChange(base + s.insert + trail);
    setOpen(false);
    setTimeout(() => inputRef.current?.focus(), 0);
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (!open || filtered.length === 0) {
      if (e.key === " " || e.key === "$" || e.key === ".") {
        setOpen(true);
      }
      return;
    }
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActiveIdx((i) => Math.min(i + 1, filtered.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActiveIdx((i) => Math.max(i - 1, 0));
    } else if (e.key === "Enter" || e.key === "Tab") {
      e.preventDefault();
      if (filtered[activeIdx]) applySuggestion(filtered[activeIdx]);
    } else if (e.key === "Escape") {
      setOpen(false);
    }
  }

  // Scroll active item into view
  useEffect(() => {
    const el = listRef.current?.children[activeIdx] as HTMLElement | undefined;
    el?.scrollIntoView({ block: "nearest" });
  }, [activeIdx]);

  return (
    <div>
      {label && <label className="block text-sm font-medium mb-1">{label}</label>}
      {desc && <p className="text-xs text-muted-foreground mb-1.5">{desc}</p>}
      <div className="relative">
        <input
          ref={inputRef}
          type="text"
          value={value}
          placeholder={placeholder ?? "e.g. $authenticated && $req.user.id == $doc.owner_id"}
          onChange={(e) => {
            onChange(e.target.value);
            setOpen(true);
          }}
          onFocus={() => setOpen(true)}
          onBlur={() => setTimeout(() => setOpen(false), 150)}
          onKeyDown={handleKeyDown}
          className="w-full rounded-md border bg-background px-3 py-2 text-sm font-mono outline-none focus:ring-2 focus:ring-ring"
          autoComplete="off"
          spellCheck={false}
        />

        {open && filtered.length > 0 && (
          <div
            ref={listRef}
            className="absolute z-50 left-0 right-0 top-full mt-1 rounded-lg border bg-popover shadow-lg max-h-64 overflow-y-auto"
          >
            {filtered.map((s, i) => (
              <div
                key={s.insert}
                onMouseDown={(e) => { e.preventDefault(); applySuggestion(s); }}
                className={`flex items-start gap-3 px-3 py-2 cursor-pointer transition-colors ${
                  i === activeIdx ? "bg-accent" : "hover:bg-accent/50"
                }`}
              >
                <span className={`shrink-0 mt-0.5 rounded px-1.5 py-0.5 text-[10px] font-mono font-semibold ${CATEGORY_COLORS[s.category]}`}>
                  {s.category}
                </span>
                <div className="min-w-0">
                  <p className="text-xs font-mono font-medium truncate">{s.insert}</p>
                  <p className="text-xs text-muted-foreground truncate">{s.detail}</p>
                </div>
              </div>
            ))}
            <div className="border-t px-3 py-1.5 text-[10px] text-muted-foreground flex gap-3">
              <span>↑↓ navigate</span>
              <span>Enter/Tab to apply</span>
              <span>Esc to close</span>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
