import { useState, useRef } from "react";

type FieldType = "text" | "number" | "boolean" | "file" | "date" | "array" | "object";

interface Field {
  id: string;
  key: string;
  type: FieldType;
  value: unknown;
}

interface Props {
  onChange: (data: Record<string, unknown>, files: Record<string, File>) => void;
  initialFields?: Field[];
}

const TYPE_OPTIONS: { value: FieldType; label: string; icon: string }[] = [
  { value: "text", label: "Text", icon: "T" },
  { value: "number", label: "Number", icon: "#" },
  { value: "boolean", label: "Yes/No", icon: "✓" },
  { value: "date", label: "Date", icon: "📅" },
  { value: "array", label: "List", icon: "[]" },
  { value: "object", label: "Object (JSON)", icon: "{}" },
  { value: "file", label: "File", icon: "📎" },
];

function uid() {
  return Math.random().toString(36).slice(2);
}

function defaultValue(type: FieldType): unknown {
  switch (type) {
    case "number": return 0;
    case "boolean": return false;
    case "array": return [];
    case "object": return {};
    case "file": return null;
    case "date": return "";
    default: return "";
  }
}

export function DynamicForm({ onChange, initialFields }: Props) {
  const [fields, setFields] = useState<Field[]>(initialFields ?? []);
  const [files, setFiles] = useState<Record<string, File>>({});

  function notify(nextFields: Field[], nextFiles: Record<string, File>) {
    const data: Record<string, unknown> = {};
    for (const f of nextFields) {
      if (f.key.trim() === "" || f.type === "file") continue;
      data[f.key.trim()] = f.value;
    }
    onChange(data, nextFiles);
  }

  function addField() {
    const next = [...fields, { id: uid(), key: "", type: "text" as FieldType, value: "" }];
    setFields(next);
    notify(next, files);
  }

  function removeField(id: string) {
    const next = fields.filter((f) => f.id !== id);
    const nextFiles = { ...files };
    const removed = fields.find((f) => f.id === id);
    if (removed?.key) delete nextFiles[removed.key];
    setFiles(nextFiles);
    setFields(next);
    notify(next, nextFiles);
  }

  function updateField(id: string, changes: Partial<Field>) {
    const next = fields.map((f) => {
      if (f.id !== id) return f;
      const updated = { ...f, ...changes };
      if (changes.type && changes.type !== f.type) {
        updated.value = defaultValue(changes.type);
      }
      return updated;
    });
    setFields(next);
    notify(next, files);
  }

  function updateFile(id: string, key: string, file: File | null) {
    const nextFiles = { ...files };
    // Remove old key if renamed
    const old = fields.find((f) => f.id === id);
    if (old?.key && old.key !== key) delete nextFiles[old.key];
    if (file && key.trim()) nextFiles[key.trim()] = file;
    setFiles(nextFiles);
    notify(fields, nextFiles);
  }

  return (
    <div className="space-y-3">
      {fields.length === 0 && (
        <p className="text-xs text-muted-foreground text-center py-4 border rounded-lg border-dashed">
          No fields yet. Click "Add field" to start building your data.
        </p>
      )}

      {fields.map((field) => (
        <FieldRow
          key={field.id}
          field={field}
          onUpdate={(changes) => updateField(field.id, changes)}
          onRemove={() => removeField(field.id)}
          onFile={(key, file) => updateFile(field.id, key, file)}
        />
      ))}

      <button
        type="button"
        onClick={addField}
        className="w-full rounded-md border border-dashed py-2 text-sm text-muted-foreground hover:border-primary hover:text-primary transition-colors"
      >
        + Add field
      </button>
    </div>
  );
}

function FieldRow({
  field,
  onUpdate,
  onRemove,
  onFile,
}: {
  field: Field;
  onUpdate: (c: Partial<Field>) => void;
  onRemove: () => void;
  onFile: (key: string, file: File | null) => void;
}) {
  const fileRef = useRef<HTMLInputElement>(null);
  const [fileName, setFileName] = useState("");
  const [arrayRaw, setArrayRaw] = useState(
    Array.isArray(field.value) ? (field.value as unknown[]).join(", ") : ""
  );
  const [objRaw, setObjRaw] = useState(
    field.type === "object" ? JSON.stringify(field.value ?? {}, null, 2) : "{}"
  );
  const [objError, setObjError] = useState("");

  return (
    <div className="flex gap-2 items-start rounded-lg border bg-card p-3">
      {/* Key name */}
      <input
        type="text"
        placeholder="field name"
        value={field.key}
        onChange={(e) => onUpdate({ key: e.target.value })}
        className="w-32 shrink-0 rounded-md border bg-background px-2 py-1.5 text-sm outline-none focus:ring-2 focus:ring-ring font-mono"
      />

      {/* Type picker */}
      <select
        value={field.type}
        onChange={(e) => onUpdate({ type: e.target.value as FieldType })}
        className="shrink-0 rounded-md border bg-background px-2 py-1.5 text-sm outline-none focus:ring-2 focus:ring-ring"
      >
        {TYPE_OPTIONS.map((t) => (
          <option key={t.value} value={t.value}>{t.icon} {t.label}</option>
        ))}
      </select>

      {/* Value input — changes based on type */}
      <div className="flex-1 min-w-0">
        {field.type === "text" && (
          <input
            type="text"
            placeholder="value"
            value={field.value as string}
            onChange={(e) => onUpdate({ value: e.target.value })}
            className="w-full rounded-md border bg-background px-2 py-1.5 text-sm outline-none focus:ring-2 focus:ring-ring"
          />
        )}

        {field.type === "number" && (
          <input
            type="number"
            placeholder="0"
            value={field.value as number}
            onChange={(e) => onUpdate({ value: parseFloat(e.target.value) || 0 })}
            className="w-full rounded-md border bg-background px-2 py-1.5 text-sm outline-none focus:ring-2 focus:ring-ring"
          />
        )}

        {field.type === "boolean" && (
          <div className="flex gap-3 pt-1">
            {[true, false].map((v) => (
              <label key={String(v)} className="flex items-center gap-1.5 cursor-pointer text-sm">
                <input
                  type="radio"
                  name={`bool-${field.id}`}
                  checked={field.value === v}
                  onChange={() => onUpdate({ value: v })}
                  className="accent-primary"
                />
                {v ? "Yes (true)" : "No (false)"}
              </label>
            ))}
          </div>
        )}

        {field.type === "date" && (
          <input
            type="datetime-local"
            value={field.value as string}
            onChange={(e) => onUpdate({ value: e.target.value })}
            className="w-full rounded-md border bg-background px-2 py-1.5 text-sm outline-none focus:ring-2 focus:ring-ring"
          />
        )}

        {field.type === "array" && (
          <div>
            <input
              type="text"
              placeholder='item1, item2, item3  (or  "a","b","c" for strings)'
              value={arrayRaw}
              onChange={(e) => {
                setArrayRaw(e.target.value);
                const items = e.target.value
                  .split(",")
                  .map((s) => s.trim())
                  .filter(Boolean)
                  .map((s) => {
                    const n = Number(s);
                    if (!isNaN(n) && s !== "") return n;
                    if (s === "true") return true;
                    if (s === "false") return false;
                    return s.replace(/^["']|["']$/g, "");
                  });
                onUpdate({ value: items });
              }}
              className="w-full rounded-md border bg-background px-2 py-1.5 text-sm outline-none focus:ring-2 focus:ring-ring"
            />
            <p className="mt-0.5 text-xs text-muted-foreground">Comma-separated. Numbers and true/false are auto-detected.</p>
          </div>
        )}

        {field.type === "object" && (
          <div>
            <textarea
              rows={4}
              value={objRaw}
              spellCheck={false}
              onChange={(e) => {
                setObjRaw(e.target.value);
                try {
                  const parsed = JSON.parse(e.target.value);
                  onUpdate({ value: parsed });
                  setObjError("");
                } catch {
                  setObjError("Invalid JSON");
                }
              }}
              className="w-full rounded-md border bg-background px-2 py-1.5 font-mono text-xs outline-none focus:ring-2 focus:ring-ring resize-y"
            />
            {objError && <p className="text-xs text-destructive mt-0.5">{objError}</p>}
          </div>
        )}

        {field.type === "file" && (
          <div>
            <input
              ref={fileRef}
              type="file"
              className="hidden"
              onChange={(e) => {
                const f = e.target.files?.[0] ?? null;
                if (f) {
                  setFileName(f.name);
                  onFile(field.key, f);
                }
              }}
            />
            <button
              type="button"
              onClick={() => fileRef.current?.click()}
              className="rounded-md border px-3 py-1.5 text-sm hover:bg-muted transition-colors"
            >
              {fileName ? `📎 ${fileName}` : "Choose file…"}
            </button>
            {fileName && (
              <button type="button" onClick={() => { setFileName(""); onFile(field.key, null); }}
                className="ml-2 text-xs text-destructive hover:underline">Remove</button>
            )}
          </div>
        )}
      </div>

      {/* Remove field */}
      <button type="button" onClick={onRemove}
        className="shrink-0 text-muted-foreground hover:text-destructive transition-colors px-1 text-lg leading-none mt-1">
        ×
      </button>
    </div>
  );
}
