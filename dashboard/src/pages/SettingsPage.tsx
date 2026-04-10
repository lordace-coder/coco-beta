import { useState, useEffect } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { configApi, type ConfigEntry } from "@/api/client";

const SMTP_KEYS = [
  { key: "smtp.host", label: "SMTP Host", placeholder: "smtp.gmail.com" },
  { key: "smtp.port", label: "SMTP Port", placeholder: "587" },
  { key: "smtp.username", label: "Username", placeholder: "you@gmail.com" },
  { key: "smtp.password", label: "Password", placeholder: "••••••••", secret: true },
  { key: "smtp.from", label: "From Email", placeholder: "noreply@yourapp.com" },
  { key: "smtp.from_name", label: "From Name", placeholder: "Your App" },
  { key: "smtp.secure", label: "TLS (true/false)", placeholder: "false" },
];

export default function SettingsPage() {
  const qc = useQueryClient();
  const [values, setValues] = useState<Record<string, string>>({});
  const [testEmail, setTestEmail] = useState("");
  const [testResult, setTestResult] = useState<{ ok: boolean; msg: string } | null>(null);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);

  const { data } = useQuery({
    queryKey: ["config"],
    queryFn: () => configApi.list().then((r) => r.data),
  });

  useEffect(() => {
    if (!data) return;
    const map: Record<string, string> = {};
    (data.data as ConfigEntry[]).forEach((c) => {
      map[c.key] = c.value;
    });
    setValues(map);
  }, [data]);

  const updateMutation = useMutation({
    mutationFn: (items: { key: string; value: string }[]) => configApi.update(items),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["config"] }),
  });

  async function handleSave(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    try {
      const items = SMTP_KEYS.map((k) => ({ key: k.key, value: values[k.key] ?? "" }));
      await updateMutation.mutateAsync(items);
    } finally {
      setSaving(false);
    }
  }

  async function handleTest(e: React.FormEvent) {
    e.preventDefault();
    setTesting(true);
    setTestResult(null);
    try {
      await configApi.testSmtp(testEmail);
      setTestResult({ ok: true, msg: "Test email sent successfully!" });
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { message?: string } } })?.response?.data?.message;
      setTestResult({ ok: false, msg: msg ?? "Test failed" });
    } finally {
      setTesting(false);
    }
  }

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-bold">Settings</h1>
        <p className="text-muted-foreground">Configure SMTP and other runtime settings.</p>
      </div>

      {/* SMTP */}
      <div className="rounded-lg border bg-card">
        <div className="border-b px-4 py-3 font-semibold">SMTP Configuration</div>
        <form onSubmit={handleSave} className="space-y-4 p-4">
          <div className="grid gap-4 sm:grid-cols-2">
            {SMTP_KEYS.map((field) => (
              <div key={field.key}>
                <label className="mb-1 block text-sm font-medium">{field.label}</label>
                <input
                  type={field.secret ? "password" : "text"}
                  placeholder={field.placeholder}
                  value={values[field.key] ?? ""}
                  onChange={(e) => setValues((v) => ({ ...v, [field.key]: e.target.value }))}
                  className="w-full rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring"
                />
              </div>
            ))}
          </div>
          <button
            type="submit"
            disabled={saving}
            className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50"
          >
            {saving ? "Saving…" : "Save SMTP settings"}
          </button>
        </form>
      </div>

      {/* Test SMTP */}
      <div className="rounded-lg border bg-card">
        <div className="border-b px-4 py-3 font-semibold">Test SMTP</div>
        <form onSubmit={handleTest} className="flex gap-2 p-4">
          <input
            type="email"
            required
            placeholder="send test to…"
            value={testEmail}
            onChange={(e) => setTestEmail(e.target.value)}
            className="flex-1 rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring"
          />
          <button
            type="submit"
            disabled={testing}
            className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50"
          >
            {testing ? "Sending…" : "Send test"}
          </button>
        </form>
        {testResult && (
          <p className={`px-4 pb-4 text-sm ${testResult.ok ? "text-green-600" : "text-destructive"}`}>
            {testResult.msg}
          </p>
        )}
      </div>
    </div>
  );
}
