import { useState } from "react";

interface EnvVar {
  key: string;
  example?: string;
  desc: string;
  required?: boolean;
}

interface Section {
  title: string;
  desc: string;
  vars: EnvVar[];
}

const SECTIONS: Section[] = [
  {
    title: "Core",
    desc: "Required settings to run Cocobase.",
    vars: [
      { key: "DATABASE_URL", example: "postgresql://user:pass@localhost:5432/mydb", desc: "PostgreSQL connection string. Also accepts a SQLite path like ./cocobase.db or sqlite:///data/cocobase.db.", required: true },
      { key: "SECRET", example: "a-long-random-string-32chars+", desc: "JWT signing secret. Keep this private and change before going to production.", required: true },
      { key: "PORT", example: "3000", desc: "HTTP port the server listens on. Defaults to 3000." },
      { key: "ENVIRONMENT", example: "production", desc: "Set to 'production' to disable debug output. Defaults to development." },
    ],
  },
  {
    title: "Email / SMTP",
    desc: "Needed for email verification, password reset and welcome emails. Leave blank to disable emails.",
    vars: [
      { key: "SMTP_HOST", example: "smtp.gmail.com", desc: "SMTP server hostname." },
      { key: "SMTP_PORT", example: "587", desc: "SMTP port. 587 for STARTTLS, 465 for SSL. Defaults to 587." },
      { key: "SMTP_USERNAME", example: "you@gmail.com", desc: "SMTP login username." },
      { key: "SMTP_PASSWORD", example: "app-password", desc: "SMTP login password or app password." },
      { key: "SMTP_FROM", example: "no-reply@yourapp.com", desc: "From address shown in sent emails." },
      { key: "SMTP_FROM_NAME", example: "My App", desc: "Display name in the From field. Defaults to Cocobase." },
      { key: "SMTP_SECURE", example: "true", desc: "Set to true for SSL/TLS (port 465). Leave false for STARTTLS." },
    ],
  },
  {
    title: "File Storage (Backblaze B2 / S3)",
    desc: "Optional. Required only if you use file upload features in your collections.",
    vars: [
      { key: "BACKBLAZE_KEY_ID", example: "0051234abcdef", desc: "Backblaze B2 key ID (or S3 access key)." },
      { key: "BACKBLAZE_APPLICATION_KEY", example: "K001abc...", desc: "Backblaze B2 application key (or S3 secret key)." },
      { key: "BACKBLAZE_KEY_NAME", example: "my-key", desc: "Key name label (informational)." },
      { key: "BUCKET_NAME", example: "my-uploads", desc: "Name of the B2 bucket to store files in." },
      { key: "BUCKET_ENDPOINT", example: "https://s3.us-west-004.backblazeb2.com", desc: "B2 S3-compatible endpoint URL for your region." },
    ],
  },
  {
    title: "Redis (optional)",
    desc: "Needed for real-time features and session caching. Safe to omit if you don't use those features.",
    vars: [
      { key: "REDIS_URL", example: "redis://localhost:6379", desc: "Redis connection URL. Leave blank to disable Redis." },
    ],
  },
  {
    title: "OAuth (optional)",
    desc: "Enable social login for your app users. Only fill in the providers you use.",
    vars: [
      { key: "GOOGLE_CLIENT_ID", example: "123456.apps.googleusercontent.com", desc: "Google OAuth client ID." },
      { key: "GOOGLE_CLIENT_SECRET", example: "GOCSPX-...", desc: "Google OAuth client secret." },
      { key: "GITHUB_CLIENT_ID", example: "Iv1.abc123", desc: "GitHub OAuth app client ID." },
      { key: "GITHUB_CLIENT_SECRET", example: "ghp_...", desc: "GitHub OAuth app client secret." },
      { key: "APPLE_CLIENT_ID", example: "com.yourapp.service", desc: "Apple Sign In service ID." },
      { key: "APPLE_TEAM_ID", example: "ABC123DEF", desc: "Apple developer team ID." },
      { key: "APPLE_KEY_ID", example: "KEYID12345", desc: "Apple Sign In key ID." },
      { key: "APPLE_PRIVATE_KEY", example: "-----BEGIN PRIVATE KEY-----\\n...", desc: "Apple private key contents (PEM format, with \\n for newlines)." },
    ],
  },
  {
    title: "Rate Limiting (optional)",
    desc: "Protect your API from abuse.",
    vars: [
      { key: "RATE_LIMIT_REQUESTS", example: "100", desc: "Maximum requests per window. Set to 0 to disable. Default: 0 (unlimited)." },
      { key: "RATE_LIMIT_WINDOW", example: "60", desc: "Rate limit window in seconds. Default: 60." },
    ],
  },
];

export default function EnvSetupPage() {
  const [copied, setCopied] = useState(false);

  const allVars = SECTIONS.flatMap((s) => s.vars);
  const envFile = allVars
    .map((v) => (v.example ? `${v.key}=${v.example}` : `# ${v.key}=`))
    .join("\n");

  function copyEnv() {
    navigator.clipboard.writeText(envFile).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  }

  return (
    <div className="space-y-6 max-w-3xl">
      <div>
        <h1 className="text-2xl font-bold">Environment Setup</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Create a <code className="font-mono bg-muted px-1 rounded">.env</code> file in the same directory as the Cocobase binary.
          Only <span className="font-medium text-foreground">DATABASE_URL</span> and <span className="font-medium text-foreground">SECRET</span> are required to get started.
        </p>
      </div>

      {/* Quick copy */}
      <div className="rounded-lg border bg-card p-4">
        <div className="flex items-center justify-between mb-2">
          <p className="text-sm font-medium">Template .env file</p>
          <button
            onClick={copyEnv}
            className="rounded-md bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground hover:opacity-90"
          >
            {copied ? "Copied!" : "Copy all"}
          </button>
        </div>
        <pre className="overflow-x-auto rounded-md bg-muted p-3 text-xs font-mono leading-relaxed whitespace-pre">
          {envFile}
        </pre>
      </div>

      {/* SQLite note */}
      <div className="rounded-lg border border-blue-200 bg-blue-50 p-4 text-sm">
        <p className="font-semibold text-blue-800 mb-1">Using SQLite (no PostgreSQL needed)</p>
        <p className="text-blue-700">
          Set <code className="font-mono bg-blue-100 px-1 rounded">DATABASE_URL=./cocobase.db</code> and Cocobase will use a local SQLite file.
          Great for self-hosted deployments on a single server. Use PostgreSQL for high-traffic apps or when running multiple instances.
        </p>
      </div>

      {/* Sections */}
      {SECTIONS.map((section) => (
        <div key={section.title} className="rounded-lg border bg-card">
          <div className="border-b px-4 py-3">
            <h2 className="font-semibold">{section.title}</h2>
            <p className="text-xs text-muted-foreground mt-0.5">{section.desc}</p>
          </div>
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/30 text-left text-xs text-muted-foreground">
                <th className="px-4 py-2 w-56">Variable</th>
                <th className="px-4 py-2">Description</th>
                <th className="px-4 py-2 w-48">Example value</th>
              </tr>
            </thead>
            <tbody>
              {section.vars.map((v) => (
                <tr key={v.key} className="border-b last:border-0">
                  <td className="px-4 py-2.5 align-top">
                    <code className="font-mono text-xs bg-muted px-1.5 py-0.5 rounded">{v.key}</code>
                    {v.required && (
                      <span className="ml-1.5 rounded-full bg-red-100 px-1.5 py-0.5 text-xs font-medium text-red-600">required</span>
                    )}
                  </td>
                  <td className="px-4 py-2.5 align-top text-xs text-muted-foreground">{v.desc}</td>
                  <td className="px-4 py-2.5 align-top">
                    {v.example && (
                      <code className="text-xs font-mono text-foreground break-all">{v.example}</code>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ))}

      {/* How to apply */}
      <div className="rounded-lg border bg-card p-4 space-y-2">
        <h2 className="font-semibold">How to apply</h2>
        <ol className="list-decimal list-inside space-y-1.5 text-sm text-muted-foreground">
          <li>Create <code className="font-mono bg-muted px-1 rounded">.env</code> next to your binary (or in the project root when using <code className="font-mono bg-muted px-1 rounded">air</code>).</li>
          <li>Paste the variables you need and fill in real values.</li>
          <li>Restart Cocobase — changes take effect on the next boot.</li>
          <li>On platforms like Railway, Render, or Fly.io, set these as environment variables in the platform's settings panel instead of a file.</li>
        </ol>
      </div>
    </div>
  );
}
