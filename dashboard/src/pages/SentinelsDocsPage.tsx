import { useState } from "react";
import { SentinelInput } from "@/components/SentinelInput";

export default function SentinelsDocsPage() {
  const [tryExpr, setTryExpr] = useState("");
  return (
    <div className="max-w-3xl space-y-8">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Sentinels</h1>
        <p className="text-muted-foreground mt-2">
          Dynamic, per-request security expressions for your collections — the easiest way to add ownership checks, role-based data filtering, and field-level access control without writing a single line of backend code.
        </p>
      </div>

      {/* Overview */}
      <Section title="What are Sentinels?">
        <p>
          A <strong>Sentinel</strong> is an expression you write once in the collection settings. Cocobase evaluates it on every matching request using the authenticated user and the document being accessed.
        </p>
        <ul className="list-disc list-inside space-y-1 text-sm mt-3">
          <li><strong>List sentinel</strong> — filters documents from list queries. Documents that don't match are silently excluded.</li>
          <li><strong>View sentinel</strong> — controls single-document access. Returns 403 if denied.</li>
          <li><strong>Create sentinel</strong> — runs before a new document is saved. <code className="bg-muted px-1 rounded">$doc.*</code> refers to the new data.</li>
          <li><strong>Update sentinel</strong> — runs before saving changes. Returns 403 if the current user doesn't own the doc.</li>
          <li><strong>Delete sentinel</strong> — runs before deletion. Returns 403 if denied.</li>
        </ul>
      </Section>

      {/* Variables */}
      <Section title="Available variables">
        <table className="w-full text-sm border rounded-lg overflow-hidden">
          <thead>
            <tr className="bg-muted text-left">
              <th className="px-4 py-2 font-medium">Variable</th>
              <th className="px-4 py-2 font-medium">Type</th>
              <th className="px-4 py-2 font-medium">Description</th>
            </tr>
          </thead>
          <tbody>
            {[
              ["$req.user.id", "string", "Authenticated user's ID. null if unauthenticated."],
              ["$req.user.email", "string", "Authenticated user's email."],
              ["$req.user.roles", "string[]", "Array of role strings the user has."],
              ["$req.user.<field>", "any", "Any custom field from the user's data map."],
              ["$doc.<field>", "any", "Any field from the document being accessed/created/updated/deleted."],
            ].map(([v, t, d]) => (
              <tr key={v} className="border-t">
                <td className="px-4 py-2 font-mono text-xs">{v}</td>
                <td className="px-4 py-2 text-xs text-muted-foreground">{t}</td>
                <td className="px-4 py-2 text-xs">{d}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </Section>

      {/* Operators */}
      <Section title="Operators">
        <table className="w-full text-sm border rounded-lg overflow-hidden">
          <thead>
            <tr className="bg-muted text-left">
              <th className="px-4 py-2 font-medium">Operator</th>
              <th className="px-4 py-2 font-medium">Description</th>
              <th className="px-4 py-2 font-medium">Example</th>
            </tr>
          </thead>
          <tbody>
            {[
              ["==", "Equal (loose — compares string representations)", "$req.user.id == $doc.owner_id"],
              ["!=", "Not equal", "$doc.status != \"deleted\""],
              ["<  >  <=  >=", "Numeric comparison", "$doc.age >= 18"],
              ["contains", "String/array contains value", "$req.user.roles contains \"admin\""],
              ["&&", "Logical AND", "$doc.published == true && $req.user.id == $doc.author_id"],
              ["||", "Logical OR", "$req.user.roles contains \"admin\" || $req.user.id == $doc.owner_id"],
              ["!", "Logical NOT", "!$doc.deleted"],
              ["( )", "Grouping", "($doc.a == 1 || $doc.b == 2) && $req.user.id == $doc.owner_id"],
            ].map(([op, desc, ex]) => (
              <tr key={op} className="border-t">
                <td className="px-4 py-2 font-mono text-xs">{op}</td>
                <td className="px-4 py-2 text-xs">{desc}</td>
                <td className="px-4 py-2 font-mono text-xs text-muted-foreground">{ex}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </Section>

      {/* Examples */}
      <Section title="Common patterns">
        <div className="space-y-4">
          <ExampleCard
            title="Ownership — only the creator can edit/delete"
            sentinel="update / delete"
            expr="$req.user.id == $doc.owner_id"
            explanation="Deny any user who isn't the document's owner. Works great for posts, comments, profiles."
          />
          <ExampleCard
            title="Admins bypass all restrictions"
            sentinel="list / view / update / delete"
            expr='$req.user.roles contains "admin" || $req.user.id == $doc.owner_id'
            explanation="Allow admins to see and modify everything, while regular users are limited to their own documents."
          />
          <ExampleCard
            title="Public read, authenticated write"
            sentinel="create"
            expr="$req.user.id != null"
            explanation="Anyone can read but only signed-in users can create documents."
          />
          <ExampleCard
            title="Filter unpublished posts"
            sentinel="list / view"
            expr='$doc.published == true || $req.user.id == $doc.author_id'
            explanation="Draft posts are invisible to other users in list results and direct fetches."
          />
          <ExampleCard
            title="Age-gated content"
            sentinel="list / view"
            expr="$req.user.age >= 18"
            explanation="Only users with age >= 18 in their profile data can see these documents."
          />
          <ExampleCard
            title="Prevent deleting active records"
            sentinel="delete"
            expr='$doc.status != "active"'
            explanation="Documents with status=active cannot be deleted via the API."
          />
        </div>
      </Section>

      {/* Behavior notes */}
      <Section title="Behaviour notes">
        <ul className="list-disc list-inside space-y-2 text-sm">
          <li>An <strong>empty sentinel</strong> means no restriction — all access is granted.</li>
          <li>If a sentinel expression <strong>fails to evaluate</strong> (syntax error), access is denied (fail-closed).</li>
          <li>The <strong>list sentinel</strong> filters documents <em>after</em> they are fetched from the database. It does not change query performance but does exclude documents from results silently. For large collections, combine with role-based Permissions for best performance.</li>
          <li>Sentinels run <strong>after</strong> role-based Permissions checks. If the user can't access the collection at all, the sentinel is never evaluated.</li>
          <li>In the <strong>create sentinel</strong>, <code className="bg-muted px-1 rounded text-xs">$doc.*</code> refers to the data being submitted, not a stored document.</li>
          <li>Sentinels are evaluated server-side — they cannot be bypassed from the client.</li>
          <li>If <code className="bg-muted px-1 rounded text-xs">$req.user</code> is null (unauthenticated request), any expression that references user fields will return false.</li>
        </ul>
      </Section>

      {/* Live editor */}
      <Section title="Try the editor">
        <p className="text-muted-foreground mb-3">Type a sentinel expression below — autocomplete suggestions appear as you type. Use ↑↓ to navigate, Enter or Tab to apply.</p>
        <SentinelInput
          value={tryExpr}
          onChange={setTryExpr}
          placeholder="Start typing — try $authenticated or $req.user..."
        />
        {tryExpr && (
          <div className="mt-3 rounded-md bg-muted px-3 py-2">
            <p className="text-xs text-muted-foreground mb-1">Expression</p>
            <code className="text-sm font-mono">{tryExpr}</code>
          </div>
        )}
      </Section>

      {/* Setting up */}
      <Section title="How to set up Sentinels">
        <ol className="list-decimal list-inside space-y-2 text-sm">
          <li>Open the dashboard.</li>
          <li>Click on a collection name to open the collection detail page.</li>
          <li>Switch to the <strong>Settings</strong> tab.</li>
          <li>Scroll to the <strong>Sentinels</strong> section.</li>
          <li>Enter your expressions and click <strong>Save settings</strong>.</li>
        </ol>
        <p className="text-xs text-muted-foreground mt-3">
          You can also set sentinels via the API when creating or updating a collection:
        </p>
        <pre className="mt-2 rounded-md bg-muted px-4 py-3 text-xs font-mono overflow-x-auto">
{`PATCH /collections/:id
{
  "sentinels": {
    "list": "$doc.published == true || $req.user.id == $doc.author_id",
    "update": "$req.user.id == $doc.owner_id",
    "delete": "$req.user.roles contains \\"admin\\""
  }
}`}
        </pre>
      </Section>
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="space-y-3">
      <h2 className="text-xl font-semibold">{title}</h2>
      <div className="text-sm leading-relaxed">{children}</div>
    </section>
  );
}

function ExampleCard({ title, sentinel, expr, explanation }: {
  title: string;
  sentinel: string;
  expr: string;
  explanation: string;
}) {
  return (
    <div className="rounded-lg border bg-card p-4 space-y-2">
      <div className="flex items-start justify-between gap-2">
        <p className="font-medium text-sm">{title}</p>
        <span className="shrink-0 rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground font-mono">{sentinel}</span>
      </div>
      <code className="block rounded-md bg-muted px-3 py-2 text-xs font-mono">{expr}</code>
      <p className="text-xs text-muted-foreground">{explanation}</p>
    </div>
  );
}
