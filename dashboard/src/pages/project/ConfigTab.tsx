import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { projectsApi } from "@/api/client";
import type { Project } from "@/api/client";

export function ConfigTab({ project }: { project: Project }) {
  const qc = useQueryClient();
  const [saved, setSaved] = useState(false);

  const cfg = (project.configs ?? {}) as Record<string, unknown>;

  const [form, setForm] = useState({
    ENABLE_EMAIL_VERIFICATION: !!cfg.ENABLE_EMAIL_VERIFICATION,
    SEND_WELCOME_EMAIL: cfg.SEND_WELCOME_EMAIL !== false,
    ENABLE_2FA: !!cfg.ENABLE_2FA,
    ENABLE_PHONE_LOGIN: !!cfg.ENABLE_PHONE_LOGIN,
    OTP_LENGTH: String(cfg.OTP_LENGTH ?? "6"),
    VERIFICATION_URL: String(cfg.VERIFICATION_URL ?? ""),
    VERIFICATION_SUCCESS_URL: String(cfg.VERIFICATION_SUCCESS_URL ?? ""),
    PASSWORD_RESET_URL: String(cfg.PASSWORD_RESET_URL ?? ""),
    LOGIN_URL: String(cfg.LOGIN_URL ?? ""),
    FRONTEND_URL: String(cfg.FRONTEND_URL ?? ""),
    ALLOWED_SELF_ROLES: Array.isArray(cfg.ALLOWED_SELF_ROLES)
      ? (cfg.ALLOWED_SELF_ROLES as string[]).join(", ")
      : String(cfg.ALLOWED_SELF_ROLES ?? ""),
    // SMTP
    smtp_host: String(cfg.smtp_host ?? ""),
    smtp_port: String(cfg.smtp_port ?? ""),
    smtp_username: String(cfg.smtp_username ?? ""),
    smtp_password: String(cfg.smtp_password ?? ""),
    smtp_from: String(cfg.smtp_from ?? ""),
    smtp_from_name: String(cfg.smtp_from_name ?? ""),
    smtp_secure: String(cfg.smtp_secure ?? "false"),
    // Resend
    resend_api_key: String(cfg.resend_api_key ?? ""),
    resend_from: String(cfg.resend_from ?? ""),
  });

  const [origins, setOrigins] = useState<string[]>(project.allowed_origins ?? []);
  const [newOrigin, setNewOrigin] = useState("");

  const updateMutation = useMutation({
    mutationFn: (body: { configs: Record<string, unknown>; allowed_origins: string[] }) =>
      projectsApi.update(project.id, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["project", project.id] });
      setSaved(true);
      setTimeout(() => setSaved(false), 3000);
    },
  });

  function handleSave(e: React.FormEvent) {
    e.preventDefault();
    const configs: Record<string, unknown> = {
      ENABLE_EMAIL_VERIFICATION: form.ENABLE_EMAIL_VERIFICATION,
      SEND_WELCOME_EMAIL: form.SEND_WELCOME_EMAIL,
      ENABLE_2FA: form.ENABLE_2FA,
      ENABLE_PHONE_LOGIN: form.ENABLE_PHONE_LOGIN,
      OTP_LENGTH: parseInt(form.OTP_LENGTH) || 6,
    };
    if (form.VERIFICATION_URL) configs.VERIFICATION_URL = form.VERIFICATION_URL;
    if (form.VERIFICATION_SUCCESS_URL) configs.VERIFICATION_SUCCESS_URL = form.VERIFICATION_SUCCESS_URL;
    if (form.PASSWORD_RESET_URL) configs.PASSWORD_RESET_URL = form.PASSWORD_RESET_URL;
    if (form.LOGIN_URL) configs.LOGIN_URL = form.LOGIN_URL;
    if (form.FRONTEND_URL) configs.FRONTEND_URL = form.FRONTEND_URL;
    if (form.ALLOWED_SELF_ROLES.trim()) {
      configs.ALLOWED_SELF_ROLES = form.ALLOWED_SELF_ROLES.split(",").map((s) => s.trim()).filter(Boolean);
    }
    // SMTP
    if (form.smtp_host) configs.smtp_host = form.smtp_host;
    if (form.smtp_port) configs.smtp_port = form.smtp_port;
    if (form.smtp_username) configs.smtp_username = form.smtp_username;
    if (form.smtp_password) configs.smtp_password = form.smtp_password;
    if (form.smtp_from) configs.smtp_from = form.smtp_from;
    if (form.smtp_from_name) configs.smtp_from_name = form.smtp_from_name;
    configs.smtp_secure = form.smtp_secure === "true";
    // Resend
    if (form.resend_api_key) configs.resend_api_key = form.resend_api_key;
    if (form.resend_from) configs.resend_from = form.resend_from;
    updateMutation.mutate({ configs, allowed_origins: origins });
  }

  function addOrigin() {
    const o = newOrigin.trim();
    if (o && !origins.includes(o)) setOrigins([...origins, o]);
    setNewOrigin("");
  }

  function toggle(key: keyof typeof form) {
    setForm((f) => ({ ...f, [key]: !f[key] }));
  }

  return (
    <form onSubmit={handleSave} className="space-y-6">
      {/* Allowed Origins */}
      <Section title="Allowed Origins" desc="Domains allowed to call this project's API. Empty = allow all.">
        <div className="flex flex-wrap gap-2 mb-3 min-h-[28px]">
          {origins.length === 0
            ? <span className="text-xs text-muted-foreground">All origins allowed</span>
            : origins.map((o) => (
              <span key={o} className="flex items-center gap-1 rounded-full bg-muted px-3 py-1 text-xs font-mono">
                {o}
                <button type="button" onClick={() => setOrigins(origins.filter((x) => x !== o))}
                  className="ml-1 text-muted-foreground hover:text-destructive font-bold">×</button>
              </span>
            ))}
        </div>
        <div className="flex gap-2">
          <input type="text" placeholder="https://yourapp.com" value={newOrigin}
            onChange={(e) => setNewOrigin(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); addOrigin(); } }}
            className="flex-1 rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring" />
          <button type="button" onClick={addOrigin} className="rounded-md border px-3 py-2 text-sm hover:bg-muted">Add</button>
        </div>
      </Section>

      {/* Auth toggles */}
      <Section title="Authentication" desc="Control which auth features are enabled for this project's users.">
        <div className="space-y-3">
          <Toggle label="Email verification" desc="Users must verify email before they can log in."
            checked={form.ENABLE_EMAIL_VERIFICATION} onChange={() => toggle("ENABLE_EMAIL_VERIFICATION")} />
          <Toggle label="Send welcome email" desc="Send a welcome email when a user signs up."
            checked={form.SEND_WELCOME_EMAIL} onChange={() => toggle("SEND_WELCOME_EMAIL")} />
          <Toggle label="Two-factor authentication" desc="Allow users to enable 2FA on their accounts."
            checked={form.ENABLE_2FA} onChange={() => toggle("ENABLE_2FA")} />
          <Toggle label="Phone login" desc="Allow users to log in via OTP sent to their phone."
            checked={form.ENABLE_PHONE_LOGIN} onChange={() => toggle("ENABLE_PHONE_LOGIN")} />
        </div>
        <div className="mt-4 grid gap-4 sm:grid-cols-2">
          <div>
            <label className="block text-sm font-medium mb-1">OTP / code length</label>
            <p className="text-xs text-muted-foreground mb-1.5">Digits in verification codes (4–10). Default: 6.</p>
            <input type="number" min={4} max={10} value={form.OTP_LENGTH}
              onChange={(e) => setForm((f) => ({ ...f, OTP_LENGTH: e.target.value }))}
              className="w-24 rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring" />
          </div>
          <div>
            <label className="block text-sm font-medium mb-1">Allowed self-assign roles</label>
            <p className="text-xs text-muted-foreground mb-1.5">Roles users can give themselves on signup/update. Leave empty to allow any.</p>
            <input type="text" placeholder="user, member, editor" value={form.ALLOWED_SELF_ROLES}
              onChange={(e) => setForm((f) => ({ ...f, ALLOWED_SELF_ROLES: e.target.value }))}
              className="w-full rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring" />
          </div>
        </div>
      </Section>

      {/* URLs */}
      <Section title="App URLs" desc="Set your frontend URLs so Cocobase generates correct links in emails.">
        <div className="grid gap-4 sm:grid-cols-2">
          <URLField label="Frontend URL" desc="Your app's base URL." placeholder="https://yourapp.com"
            value={form.FRONTEND_URL} onChange={(v) => setForm((f) => ({ ...f, FRONTEND_URL: v }))} />
          <URLField label="Login URL" desc="Included in welcome and reset emails." placeholder="https://yourapp.com/login"
            value={form.LOGIN_URL} onChange={(v) => setForm((f) => ({ ...f, LOGIN_URL: v }))} />
          <URLField label="Email verification URL" desc="Where users go to verify their email." placeholder="https://yourapp.com/verify"
            value={form.VERIFICATION_URL} onChange={(v) => setForm((f) => ({ ...f, VERIFICATION_URL: v }))} />
          <URLField label="Verification success URL" desc="Redirect here after email is verified." placeholder="https://yourapp.com/welcome"
            value={form.VERIFICATION_SUCCESS_URL} onChange={(v) => setForm((f) => ({ ...f, VERIFICATION_SUCCESS_URL: v }))} />
          <URLField label="Password reset URL" desc="Where users go to reset their password." placeholder="https://yourapp.com/reset-password"
            value={form.PASSWORD_RESET_URL} onChange={(v) => setForm((f) => ({ ...f, PASSWORD_RESET_URL: v }))} />
        </div>
      </Section>

      {/* Mailer */}
      <Section title="Email / Mailer" desc="Configure how this project sends emails. Resend takes priority over SMTP if both are set.">
        <div className="space-y-4">
          <p className="text-xs text-muted-foreground bg-muted rounded-md px-3 py-2">
            Leave fields empty to fall back to global .env SMTP settings. Set a Resend API key to use Resend instead of SMTP.
          </p>

          <div>
            <h4 className="text-sm font-semibold mb-3">Resend (recommended)</h4>
            <div className="grid gap-4 sm:grid-cols-2">
              <TextField label="Resend API key" desc="Get from resend.com. Overrides SMTP if set." type="password"
                placeholder="re_..." value={form.resend_api_key}
                onChange={(v) => setForm((f) => ({ ...f, resend_api_key: v }))} />
              <TextField label="From address" desc='e.g. "Acme <hello@acme.com>"' type="text"
                placeholder="noreply@yourdomain.com" value={form.resend_from}
                onChange={(v) => setForm((f) => ({ ...f, resend_from: v }))} />
            </div>
          </div>

          <div>
            <h4 className="text-sm font-semibold mb-3">SMTP</h4>
            <div className="grid gap-4 sm:grid-cols-2">
              <TextField label="SMTP host" desc="e.g. smtp.sendgrid.net" type="text"
                placeholder="smtp.example.com" value={form.smtp_host}
                onChange={(v) => setForm((f) => ({ ...f, smtp_host: v }))} />
              <TextField label="SMTP port" desc="Usually 587 (TLS) or 465 (SSL)." type="number"
                placeholder="587" value={form.smtp_port}
                onChange={(v) => setForm((f) => ({ ...f, smtp_port: v }))} />
              <TextField label="SMTP username" desc="Usually your email address." type="text"
                placeholder="user@example.com" value={form.smtp_username}
                onChange={(v) => setForm((f) => ({ ...f, smtp_username: v }))} />
              <TextField label="SMTP password" desc="Your SMTP password or app password." type="password"
                placeholder="••••••••" value={form.smtp_password}
                onChange={(v) => setForm((f) => ({ ...f, smtp_password: v }))} />
              <TextField label="From address" desc="Envelope from address." type="text"
                placeholder="noreply@yourdomain.com" value={form.smtp_from}
                onChange={(v) => setForm((f) => ({ ...f, smtp_from: v }))} />
              <TextField label="From name" desc="Display name shown in email clients." type="text"
                placeholder="My App" value={form.smtp_from_name}
                onChange={(v) => setForm((f) => ({ ...f, smtp_from_name: v }))} />
            </div>
            <div className="mt-3">
              <label className="block text-sm font-medium mb-1">TLS / SSL</label>
              <p className="text-xs text-muted-foreground mb-1.5">Enable for port 465 (SSL). Leave off for port 587 (STARTTLS).</p>
              <select value={form.smtp_secure} onChange={(e) => setForm((f) => ({ ...f, smtp_secure: e.target.value }))}
                className="rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring">
                <option value="false">Off (STARTTLS / port 587)</option>
                <option value="true">On (SSL / port 465)</option>
              </select>
            </div>
          </div>
        </div>
      </Section>

      <div className="flex items-center gap-3">
        <button type="submit" disabled={updateMutation.isPending}
          className="rounded-md bg-primary px-5 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50 hover:opacity-90">
          {updateMutation.isPending ? "Saving…" : "Save settings"}
        </button>
        {saved && <span className="text-sm text-green-600">Saved!</span>}
        {updateMutation.isError && <span className="text-sm text-destructive">Failed to save.</span>}
      </div>
    </form>
  );
}

function Section({ title, desc, children }: { title: string; desc: string; children: React.ReactNode }) {
  return (
    <div className="rounded-lg border bg-card p-5 space-y-4">
      <div>
        <h3 className="font-semibold">{title}</h3>
        <p className="text-xs text-muted-foreground mt-0.5">{desc}</p>
      </div>
      {children}
    </div>
  );
}

function Toggle({ label, desc, checked, onChange }: { label: string; desc: string; checked: boolean; onChange: () => void }) {
  return (
    <label className="flex items-start gap-3 cursor-pointer">
      <div className="mt-0.5 relative shrink-0">
        <input type="checkbox" className="sr-only" checked={checked} onChange={onChange} />
        <div className={`w-9 h-5 rounded-full transition-colors ${checked ? "bg-primary" : "bg-muted-foreground/30"}`}>
          <div className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white shadow transition-transform ${checked ? "translate-x-4" : ""}`} />
        </div>
      </div>
      <div>
        <p className="text-sm font-medium">{label}</p>
        <p className="text-xs text-muted-foreground">{desc}</p>
      </div>
    </label>
  );
}

function URLField({ label, desc, placeholder, value, onChange }: { label: string; desc: string; placeholder: string; value: string; onChange: (v: string) => void }) {
  return (
    <div>
      <label className="block text-sm font-medium mb-1">{label}</label>
      <p className="text-xs text-muted-foreground mb-1.5">{desc}</p>
      <input type="url" placeholder={placeholder} value={value} onChange={(e) => onChange(e.target.value)}
        className="w-full rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring" />
    </div>
  );
}

function TextField({ label, desc, placeholder, value, onChange, type = "text" }: { label: string; desc: string; placeholder: string; value: string; onChange: (v: string) => void; type?: string }) {
  return (
    <div>
      <label className="block text-sm font-medium mb-1">{label}</label>
      <p className="text-xs text-muted-foreground mb-1.5">{desc}</p>
      <input type={type} placeholder={placeholder} value={value} onChange={(e) => onChange(e.target.value)}
        className="w-full rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring" />
    </div>
  );
}
