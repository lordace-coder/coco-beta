# Cocobase Cloud Functions

Functions let you run server-side JavaScript scoped to a project.  
Each `.js` file in `./functions/<projectID>/` is a separate function.  
Files are reloaded automatically when saved — no restart needed.

---

## File layout

```
functions/
  hello_world.js   ← each file is one function
  send_email.js
  utils.js         ← shared helper (not a function — just required by others)
  cocobase.d.ts    ← auto-generated types (do not edit)
  jsconfig.json    ← enables VS Code IntelliSense
```

---

## HTTP routes

```js
// GET /functions/func/hello_world
app.get("/hello_world", (ctx) => {
  const name = ctx.req.query.name ?? "world";
  ctx.respond(200, JSON.stringify({ message: `Hello, ${name}!` }), {
    "Content-Type": "application/json",
  });
});

// POST — parse JSON body
app.post("/users/create", (ctx) => {
  const body = ctx.req.json();           // parses JSON body, returns null on failure
  const doc  = ctx.db.create("users", { email: body.email });
  ctx.respond(201, JSON.stringify(doc), { "Content-Type": "application/json" });
});

// Any method
app.all("/webhook", (ctx) => {
  ctx.log("method:", ctx.req.method, "body:", ctx.req.body);
  ctx.respond(200, "ok");
});
```

**Available methods:** `app.get`, `app.post`, `app.put`, `app.patch`, `app.delete`, `app.all`

**Base URL:** `http://localhost:3000/functions/func`

---

## Cron jobs

Standard 5-field cron expression (minute hour day month weekday).

```js
app.cron("0 9 * * 1-5", (ctx) => {   // 9am on weekdays
  ctx.log("Morning job fired at", new Date().toISOString());
});

app.cron("*/5 * * * *", (ctx) => {   // every 5 minutes
  const old = ctx.db.query("SELECT id FROM sessions WHERE expires_at < ?", new Date().toISOString());
  ctx.log("expired sessions:", old.rows.length);
});
```

Use [crontab.guru](https://crontab.guru) to build expressions.

---

## Collection hooks

Hooks fire on collection lifecycle events. Use `ctx.next()` to allow or `ctx.cancel()` to block.

```js
// Validate before create
app.on("beforeCreate", "orders", (ctx) => {
  if (!ctx.doc.amount || ctx.doc.amount <= 0) {
    ctx.cancel("amount must be positive");   // blocks the create, returns 400
  } else {
    ctx.next();                              // allow it through
  }
});

// React after create (fire-and-forget, no need to call next/cancel)
app.on("afterCreate", "orders", (ctx) => {
  ctx.log("new order:", ctx.doc.id, "amount:", ctx.doc.amount);
  ctx.mail({
    to: ctx.doc.email,
    subject: "Order confirmed",
    html: `<p>Your order #${ctx.doc.id} is confirmed.</p>`,
  });
});

// All collections (omit second arg)
app.on("beforeDelete", (ctx) => {
  if (!ctx.user) ctx.cancel("must be authenticated to delete");
  else ctx.next();
});
```

**Hook events:** `beforeCreate`, `afterCreate`, `beforeUpdate`, `afterUpdate`, `beforeDelete`, `afterDelete`

---

## App hooks

```js
app.on("start", (ctx) => {
  ctx.log("Project", ctx.project.name, "started");
});
```

---

## ctx API reference

### ctx.req — incoming request (HTTP functions)
```js
ctx.req.method       // "GET", "POST", etc.
ctx.req.path         // "/hello_world"
ctx.req.headers      // { "content-type": "application/json", ... }
ctx.req.query        // { page: "1", limit: "20" }
ctx.req.body         // raw string
ctx.req.json()       // parsed JSON body (null on failure)
```

### ctx.db — database

#### Basic CRUD
```js
ctx.db.get("posts", id)                     // one doc or null
ctx.db.create("posts", { title, body })     // insert and return doc
ctx.db.update("posts", id, { title })       // merge fields, return doc
ctx.db.delete("posts", id)                  // delete by ID
```

#### db.query(collection, options) — filtered query
```js
const result = ctx.db.query("posts", {
  status: "published",       // exact match
  limit: 20,
  offset: 0,
  sort: "created_at",
  order: "desc",             // "asc" | "desc"
});
result.data      // array of documents
result.total     // total matching count
result.has_more  // boolean
```

#### db.findOne(collection, options) — first match or null
```js
const post = ctx.db.findOne("posts", { slug: "hello-world" });
if (!post) return ctx.respond(404, "Not found");
```

#### Filter operators
Append a suffix to any field name:

| Suffix | Meaning | Example |
|---|---|---|
| *(none)* or `_eq` | exact match | `status: "published"` |
| `_ne` | not equal | `status_ne: "draft"` |
| `_gt` / `_gte` | greater / greater-or-equal | `views_gte: "100"` |
| `_lt` / `_lte` | less / less-or-equal | `price_lte: "500"` |
| `_contains` | case-insensitive substring | `title_contains: "guide"` |
| `_startswith` | starts with | `name_startswith: "Jo"` |
| `_endswith` | ends with | `email_endswith: "@gmail.com"` |
| `_in` | comma-separated list | `status_in: "published,featured"` |
| `_notin` | not in list | `category_notin: "spam,nsfw"` |
| `_isnull` | null check | `deleted_at_isnull: "true"` |

```js
// Multiple operators combined (AND by default)
const products = ctx.db.query("products", {
  price_gte: "50",
  price_lte: "500",
  stock_gt: "0",
  category_ne: "discontinued",
  limit: 50,
});
```

#### OR logic
Prefix a key with `[or]` or `[or:groupName]`:
```js
// status = "published" OR status = "featured"
const posts = ctx.db.query("posts", {
  "[or]status":   "published",
  "[or]status_2": "featured",
});

// (category=tech OR category=js) AND (status=published OR status=featured)
const posts = ctx.db.query("posts", {
  "[or:cat]category":    "tech",
  "[or:cat]category_2":  "javascript",
  "[or:st]status":       "published",
  "[or:st]status_2":     "featured",
});

// Search across multiple fields
const posts = ctx.db.query("posts", {
  "[or:search]title_contains":   "cocobase",
  "[or:search]content_contains": "cocobase",
});
```

#### db.queryUsers(options) / db.findUser(options)
Same filter syntax, queries app users for the project:
```js
const result = ctx.db.queryUsers({
  email_endswith: "@gmail.com",
  created_at_gte: "2024-01-01",
  sort: "created_at",
  order: "desc",
  limit: 50,
});

const user = ctx.db.findUser({ email: "alice@example.com" });
if (!user) return ctx.respond(404, "User not found");
```

### ctx.auth — app user management

```js
// Create a user
const user = ctx.auth.createUser({
  email: "alice@example.com",
  password: "secret123",
  data: { plan: "free", referral: "bob" },
  roles: ["member"],
});

// Get by ID
const user = ctx.auth.getUser(id);            // or null

// Find by any field (same filter syntax as db.query)
const user = ctx.auth.findUser({ email: "alice@example.com" });
const user = ctx.auth.findUser({ "data.plan": "pro" });

// Query with filters
const result = ctx.auth.queryUsers({
  email_endswith: "@gmail.com",
  sort: "created_at",
  order: "desc",
  limit: 50,
});
result.data      // array of users
result.total
result.has_more

// Update (merges — only fields you pass are changed)
ctx.auth.updateUser(id, {
  data: { plan: "pro" },
  roles: ["member", "pro"],
  emailVerified: true,
});

// Change password
ctx.auth.updateUser(id, { password: "newpassword" });

// Delete
ctx.auth.deleteUser(id);
```

### ctx.queue — background tasks

Run work **after** the response is sent to the user. Useful for emails, analytics, webhooks — anything non-critical.

- Max **100** pending tasks per project
- **20-second** timeout per task
- **Not persisted** — lost on server restart. Use cron for reliable scheduling.

```js
// queue.add(handler, data?) — inline function
app.post("/signup", (ctx) => {
  const body = ctx.req.json();
  const user = ctx.auth.createUser({ email: body.email, password: body.password });

  // Return immediately, send welcome email in background
  ctx.queue.add((data) => {
    ctx.mail({
      to: data.email,
      subject: "Welcome to the app!",
      html: `<h1>Hi ${data.email}!</h1>`,
    });
  }, { email: user.email });

  ctx.respond(201, JSON.stringify(user), { "Content-Type": "application/json" });
});

// queue.call(fileName, data?) — dispatch to app.on("queue", fileName, handler)
// in send_welcome.js:
app.on("queue", "send_welcome", (ctx) => {
  const { email, name } = ctx.doc;
  ctx.log("sending welcome to", email);
  ctx.mail({ to: email, subject: "Welcome!", html: `<p>Hi ${name}!</p>` });
});

// in signup.js:
app.post("/signup", (ctx) => {
  const user = ctx.auth.createUser({ email: ctx.req.json().email, password: ctx.req.json().password });
  ctx.queue.call("send_welcome", { email: user.email, name: user.email });
  ctx.respond(201, JSON.stringify(user), { "Content-Type": "application/json" });
});
```

### ctx.kv — key-value store
```js
ctx.kv.set("rate:123", count, 60)   // ttl in seconds (0 = forever)
ctx.kv.get("rate:123")              // value or null
ctx.kv.delete("rate:123")
```

### ctx.user — authenticated app user (or null)
```js
if (!ctx.user) return ctx.respond(401, "Unauthorized");
ctx.log("user:", ctx.user.id, ctx.user.email);
ctx.user.roles     // string[]
ctx.user.verified  // boolean
```

### ctx.fetch — external HTTP (SSRF-protected)
```js
const res = ctx.fetch("https://api.example.com/data", {
  method: "POST",
  body: JSON.stringify({ key: "value" }),
  headers: { "Content-Type": "application/json" },
});
res.status   // 200
res.body     // string
res.json()   // parsed JSON
```

> Private/localhost addresses are blocked automatically.

### ctx.mail — send email (requires SMTP config in dashboard)
```js
ctx.mail({
  to: "user@example.com",
  subject: "Welcome!",
  html: "<h1>Hello</h1>",
  text: "Hello",   // optional plain-text fallback
});
```

### ctx.publish — realtime events
```js
ctx.publish("notifications", { type: "new_message", from: ctx.user.id });
// Clients subscribed via WebSocket receive this immediately.
```

### ctx.log — execution log
```js
ctx.log("processing order", orderId);
ctx.log("doc:", ctx.doc);    // objects are JSON-serialised
// Output appears in the dashboard Run result panel.
```

---

## require() — share code between files

```js
// utils.js
module.exports = {
  json: (ctx, status, data) => {
    ctx.respond(status, JSON.stringify(data), { "Content-Type": "application/json" });
  },
};

// orders.js
const { json } = require("./utils");

app.get("/orders", (ctx) => {
  const rows = ctx.db.list("orders");
  json(ctx, 200, { data: rows });
});
```

Only relative paths (`./file` or `../file`) are supported. No npm packages.

---

## Full example

```js
// payments.js
const json = (ctx, status, data) =>
  ctx.respond(status, JSON.stringify(data), { "Content-Type": "application/json" });

// List payments (paginated, filtered)
app.get("/payments", (ctx) => {
  const { page = "1", status } = ctx.req.query;
  const limit = 20;
  const offset = (parseInt(page) - 1) * limit;

  const opts = { limit, offset, sort: "created_at", order: "desc" };
  if (status) opts.status = status;

  const result = ctx.db.query("payments", opts);
  json(ctx, 200, result);
});

// Create a payment
app.post("/payments", (ctx) => {
  const body = ctx.req.json() ?? {};
  if (!body.amount) return json(ctx, 400, { error: "amount required" });

  const payment = ctx.db.create("payments", {
    amount: body.amount,
    currency: body.currency ?? "usd",
    user_id: ctx.user?.id ?? null,
    status: "pending",
  });
  json(ctx, 201, payment);
});

// Confirm a payment
app.post("/payments/confirm", (ctx) => {
  const { id } = ctx.req.json() ?? {};
  const payment = ctx.db.get("payments", id);
  if (!payment) return json(ctx, 404, { error: "not found" });

  const updated = ctx.db.update("payments", id, { status: "confirmed" });
  ctx.publish("payments", { event: "confirmed", payment: updated });
  json(ctx, 200, updated);
});

// Search payments by user email
app.get("/payments/search", (ctx) => {
  const { email, min_amount } = ctx.req.query;
  const opts = { sort: "created_at", order: "desc", limit: 50 };
  if (min_amount) opts.amount_gte = min_amount;

  // Find user first
  if (email) {
    const user = ctx.db.findUser({ email });
    if (!user) return json(ctx, 200, { data: [], total: 0, has_more: false });
    opts.user_id = user.id;
  }

  json(ctx, 200, ctx.db.query("payments", opts));
});

// Daily cleanup — expire pending payments older than 7 days
app.cron("0 2 * * *", (ctx) => {
  const cutoff = new Date(Date.now() - 7 * 86400_000).toISOString();
  const stale = ctx.db.query("payments", {
    status: "pending",
    created_at_lt: cutoff,
    limit: 500,
  });
  stale.data.forEach(p => ctx.db.delete("payments", p.id));
  ctx.log("cleaned up", stale.data.length, "stale payments");
});

// Hook — prevent payments without an authenticated user
app.on("beforeCreate", "payments", (ctx) => {
  if (!ctx.doc.user_id) {
    ctx.cancel("user_id is required");
  } else {
    ctx.next();
  }
});
```
