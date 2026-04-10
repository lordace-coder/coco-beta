# Cloud Functions

Cocobase lets you write JavaScript functions that run server-side. No separate deployment needed — write the code in the dashboard, save, done.

There are three trigger types:

| Type | When it runs |
|------|-------------|
| `http` | Someone calls `/fn/your-path` with your API key |
| `hook` | A document in a collection is created, updated, or deleted |
| `cron` | A time schedule fires (e.g. every hour) |

---

## What a complete function looks like

Your code runs with `ctx` already in scope. You do not declare it — it is just there.

### A complete HTTP function

This is the **entire file** you would write in the dashboard for an HTTP function:

```js
// Trigger type: http
// Method: GET
// Path: /hello

const name = ctx.req.query.name || "world";

ctx.respond(200, JSON.stringify({ message: "Hello " + name }), {
  "Content-Type": "application/json"
});
```

Call it like this:

```bash
curl "http://localhost:3000/fn/hello?name=Alice" \
  -H "X-API-Key: coco_your_key"

# Response:
# { "message": "Hello Alice" }
```

---

### A complete hook function

This is the **entire file** for a hook that runs before every new document is created in the `orders` collection:

```js
// Trigger type: hook
// Event: beforeCreate
// Collection: orders

if (!ctx.doc.product_id) {
  ctx.cancel("product_id is required");
  return;
}

if (ctx.doc.quantity <= 0) {
  ctx.cancel("quantity must be a positive number");
  return;
}

// Stamp the owner automatically
ctx.doc.owner_id = ctx.user ? ctx.user.id : null;
ctx.doc.status = "pending";

ctx.next();
```

When `ctx.cancel("message")` is called, the document is **not saved** and the API returns a 400 with that message. When `ctx.next()` is called, the document saves normally.

---

### A complete cron function

This is the **entire file** for a cron that runs every day at 3am and cleans up old sessions:

```js
// Trigger type: cron
// Schedule: 0 3 * * *

const sessions = ctx.db.list("sessions");
const sevenDaysAgo = Date.now() - 7 * 24 * 60 * 60 * 1000;
let removed = 0;

for (const session of sessions) {
  if (new Date(session.created_at).getTime() < sevenDaysAgo) {
    ctx.db.delete("sessions", session.id);
    removed++;
  }
}

ctx.log("Cleaned up", removed, "expired sessions");
```

`ctx.log(...)` output appears in the dashboard under the function's recent runs list.

---

## HTTP functions in depth

### The route

HTTP functions are always mounted at `/fn/<your-path>`. The path is whatever you set in the trigger config.

```
GET  /fn/hello          →  your function runs
POST /fn/orders/submit  →  your function runs
ANY  /fn/webhook        →  your function runs for any HTTP method
```

Every request must include the project API key:

```bash
# Header (recommended)
curl /fn/hello -H "X-API-Key: coco_your_key"

# The key is the same one used for all other Cocobase API calls
```

### Reading the request

```js
// Trigger type: http
// Method: POST
// Path: /echo

// Read query params — e.g. /fn/echo?format=pretty
const format = ctx.req.query.format;

// Read a header
const authHeader = ctx.req.headers["authorization"];

// Read raw body (string)
const raw = ctx.req.body;

// Parse JSON body
const body = ctx.req.json();   // returns null if body is not valid JSON
if (!body) {
  ctx.respond(400, "Expected JSON body");
  return;
}

ctx.log("Received:", JSON.stringify(body));
ctx.respond(200, JSON.stringify({ received: body, format: format }), {
  "Content-Type": "application/json"
});
```

### Sending different response types

```js
// Trigger type: http
// Method: GET
// Path: /data

// JSON response
ctx.respond(200, JSON.stringify({ ok: true, count: 5 }), {
  "Content-Type": "application/json"
});
```

```js
// Trigger type: http
// Method: GET
// Path: /page

// HTML response
ctx.respond(200, `
<!DOCTYPE html>
<html>
  <head><title>${ctx.project.name}</title></head>
  <body>
    <h1>Hello from ${ctx.project.name}</h1>
    <p>You are: ${ctx.user ? ctx.user.email : "not logged in"}</p>
  </body>
</html>
`, { "Content-Type": "text/html" });
```

```js
// Trigger type: http
// Method: GET
// Path: /ping

// Plain text — Content-Type is auto-detected, you can omit it
ctx.respond(200, "pong");
```

```js
// Trigger type: http
// Method: GET
// Path: /redirect

// Redirect
ctx.respond(302, "", { "Location": "https://example.com" });
```

```js
// Trigger type: http
// Method: GET
// Path: /protected

// Error response
if (!ctx.user) {
  ctx.respond(401, JSON.stringify({ error: "Not authenticated" }), {
    "Content-Type": "application/json"
  });
  return;
}

ctx.respond(200, JSON.stringify({ hello: ctx.user.email }), {
  "Content-Type": "application/json"
});
```

### Defining helper functions (export style)

When you need to define reusable helpers, use the export style — return a function at the end:

```js
// Trigger type: http
// Method: GET
// Path: /products

function paginate(items, page, pageSize) {
  const start = (page - 1) * pageSize;
  return items.slice(start, start + pageSize);
}

function toPublic(doc) {
  // Strip internal fields before sending to client
  const result = {};
  for (const key in doc) {
    if (key !== "internal_notes" && key !== "cost_price") {
      result[key] = doc[key];
    }
  }
  return result;
}

return function(ctx) {
  const page = parseInt(ctx.req.query.page) || 1;
  const all = ctx.db.list("products");
  const paged = paginate(all, page, 10).map(toPublic);

  ctx.respond(200, JSON.stringify({
    data: paged,
    page: page,
    total: all.length,
    pages: Math.ceil(all.length / 10),
  }), { "Content-Type": "application/json" });
};
```

---

## Hook functions in depth

### Available events

| Event | When |
|-------|------|
| `beforeCreate` | Before a new document is saved — can cancel |
| `afterCreate` | After a new document is saved — runs in background |
| `beforeUpdate` | Before changes are saved — can cancel |
| `afterUpdate` | After changes are saved — runs in background |
| `beforeDelete` | Before a document is deleted — can cancel |
| `afterDelete` | After a document is deleted — runs in background |

### Before-hooks: allow or cancel

```js
// Trigger type: hook
// Event: beforeCreate
// Collection: posts

// Only logged-in users can create posts
if (!ctx.user) {
  ctx.cancel("You must be logged in to create a post");
  return;
}

// Enforce required fields
if (!ctx.doc.title || ctx.doc.title.trim() === "") {
  ctx.cancel("title is required");
  return;
}

// Auto-set fields
ctx.doc.author_id = ctx.user.id;
ctx.doc.published = false;
ctx.doc.slug = ctx.doc.title.toLowerCase().replace(/\s+/g, "-");

ctx.next();
```

```js
// Trigger type: hook
// Event: beforeDelete
// Collection: posts

// Prevent deleting published posts
if (ctx.doc.published === true) {
  ctx.cancel("Unpublish this post before deleting it");
  return;
}

// Only the author can delete
if (ctx.user && ctx.doc.author_id !== ctx.user.id) {
  ctx.cancel("You can only delete your own posts");
  return;
}

ctx.next();
```

### After-hooks: react to changes

After-hooks run in the background after the operation completes. They cannot cancel anything. Use them for notifications, emails, syncing external systems, etc.

```js
// Trigger type: hook
// Event: afterCreate
// Collection: orders

// Send confirmation email
ctx.mail({
  to: ctx.doc.customer_email,
  subject: "Order confirmed — #" + ctx.doc.id.slice(0, 8),
  html: `
    <h2>Thanks for your order!</h2>
    <p>Order ID: ${ctx.doc.id}</p>
    <p>Total: $${ctx.doc.total}</p>
  `,
});

// Notify via realtime so any open dashboard/app updates live
ctx.publish("orders", {
  event: "new_order",
  id: ctx.doc.id,
  total: ctx.doc.total,
});

ctx.log("Order confirmed and email sent to", ctx.doc.customer_email);
```

```js
// Trigger type: hook
// Event: afterUpdate
// Collection: tickets

// When a ticket is marked resolved, notify the requester
if (ctx.doc.status === "resolved" && ctx.doc.requester_email) {
  ctx.mail({
    to: ctx.doc.requester_email,
    subject: "Your ticket has been resolved",
    html: `<p>Ticket "${ctx.doc.title}" has been marked as resolved.</p>`,
  });
}
```

---

## Cron functions in depth

### Schedule format

```
minute  hour  day  month  weekday
  0      *     *     *       *      →  every hour at :00
  */5    *     *     *       *      →  every 5 minutes
  0      9     *     *       1      →  every Monday at 9:00am
  0      3     *     *       *      →  every day at 3:00am
  0      0     1     *       *      →  first day of every month at midnight
```

With an optional leading seconds field (6 fields):

```
second  minute  hour  day  month  weekday
  */30    *       *     *     *       *    →  every 30 seconds
  0       */5     *     *     *       *    →  every 5 minutes
```

Use [crontab.guru](https://crontab.guru) to build expressions visually.

### Weekly digest example

```js
// Trigger type: cron
// Schedule: 0 9 * * 1

// Every Monday at 9am — send a weekly stats email to admin

const users = ctx.db.list("app_users");
const orders = ctx.db.list("orders");

const oneWeekAgo = Date.now() - 7 * 24 * 60 * 60 * 1000;
const newUsers = users.filter(u => new Date(u.created_at).getTime() > oneWeekAgo);
const newOrders = orders.filter(o => new Date(o.created_at).getTime() > oneWeekAgo);
const revenue = newOrders.reduce((sum, o) => sum + (o.total || 0), 0);

ctx.mail({
  to: "admin@yourapp.com",
  subject: "Weekly digest — " + new Date().toDateString(),
  html: `
    <h2>Weekly Stats</h2>
    <ul>
      <li>New users this week: <strong>${newUsers.length}</strong></li>
      <li>New orders this week: <strong>${newOrders.length}</strong></li>
      <li>Revenue this week: <strong>$${revenue.toFixed(2)}</strong></li>
    </ul>
  `,
});

ctx.log("Weekly digest sent —", newUsers.length, "users,", newOrders.length, "orders");
```

### Ping an external service

```js
// Trigger type: cron
// Schedule: */5 * * * *

// Every 5 minutes — check if an external API is up and alert if not

const res = ctx.fetch("https://api.yourservice.com/health");

if (res.status !== 200) {
  ctx.mail({
    to: "oncall@yourapp.com",
    subject: "ALERT: external API is down (" + res.status + ")",
    html: `<p>Health check returned status ${res.status}.</p>`,
  });
  ctx.log("Alert sent — status:", res.status);
} else {
  ctx.log("Health check OK:", res.status);
}
```

---

## Calling external APIs

```js
// Trigger type: http
// Method: POST
// Path: /send-slack

// Forward a message to Slack
const body = ctx.req.json();
if (!body || !body.message) {
  ctx.respond(400, JSON.stringify({ error: "message is required" }));
  return;
}

const res = ctx.fetch("https://hooks.slack.com/services/YOUR/WEBHOOK/URL", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ text: body.message }),
});

if (res.status === 200) {
  ctx.respond(200, JSON.stringify({ ok: true }));
} else {
  ctx.respond(502, JSON.stringify({ error: "Slack returned " + res.status }));
}
```

---

## Rate limiting with KV

```js
// Trigger type: http
// Method: POST
// Path: /submit

// Allow max 5 requests per user per minute

const userId = ctx.user ? ctx.user.id : ctx.req.headers["x-forwarded-for"] || "anon";
const key = "ratelimit:" + userId;
const count = ctx.kv.get(key) || 0;

if (count >= 5) {
  ctx.respond(429, JSON.stringify({ error: "Too many requests. Try again in a minute." }), {
    "Content-Type": "application/json"
  });
  return;
}

ctx.kv.set(key, count + 1, 60);  // expires after 60 seconds

// ... rest of your logic
ctx.respond(200, JSON.stringify({ ok: true, requests_used: count + 1 }), {
  "Content-Type": "application/json"
});
```

---

## Caching expensive data with KV

```js
// Trigger type: http
// Method: GET
// Path: /exchange-rate

// Cache exchange rate for 1 hour instead of fetching every request

let rate = ctx.kv.get("usd_eur");

if (rate === null) {
  const res = ctx.fetch("https://open.er-api.com/v6/latest/USD");
  const data = res.json();
  rate = data.rates.EUR;
  ctx.kv.set("usd_eur", rate, 3600);   // cache for 1 hour
  ctx.log("Fetched fresh rate:", rate);
} else {
  ctx.log("Served cached rate:", rate);
}

ctx.respond(200, JSON.stringify({ usd_to_eur: rate }), {
  "Content-Type": "application/json"
});
```

---

## Full ctx reference

### `ctx.req`
| Property | Type | Description |
|----------|------|-------------|
| `ctx.req.method` | string | `"GET"`, `"POST"`, etc. |
| `ctx.req.path` | string | Path after `/fn`, e.g. `"/hello"` |
| `ctx.req.headers` | object | All request headers, lowercase keys |
| `ctx.req.query` | object | Query string params as key/value strings |
| `ctx.req.body` | string | Raw request body |
| `ctx.req.json()` | function | Parses body as JSON, returns `null` on failure |

### `ctx.user`
`null` if unauthenticated. Otherwise:

| Property | Type | Description |
|----------|------|-------------|
| `ctx.user.id` | string | User UUID |
| `ctx.user.email` | string | User email |
| `ctx.user.roles` | string[] | e.g. `["admin", "editor"]` |
| `ctx.user.verified` | boolean | Whether email is verified |

### `ctx.doc`
Available in hook functions. The document being operated on.

| Property | Type | Description |
|----------|------|-------------|
| `ctx.doc.id` | string | Document UUID (not set in `beforeCreate`) |
| `ctx.doc.created_at` | string | ISO timestamp |
| `ctx.doc.<field>` | any | Any field stored on the document |

You can **mutate** `ctx.doc` in before-hooks to change what gets saved.

### `ctx.project`
| Property | Type | Description |
|----------|------|-------------|
| `ctx.project.id` | string | Project UUID |
| `ctx.project.name` | string | Project name |

### `ctx.respond(status, body, headers?)`
Sends the HTTP response. Only works in HTTP functions.

### `ctx.next()` / `ctx.cancel(msg?)`
Only meaningful in before-hooks. `next()` allows the operation. `cancel(msg)` blocks it with a 400 error.

### `ctx.db`
| Method | Returns | Description |
|--------|---------|-------------|
| `ctx.db.list(collection)` | array | All documents, newest first, max 500 |
| `ctx.db.get(collection, id)` | object or null | One document by ID |
| `ctx.db.create(collection, data)` | object | Created document with new ID |
| `ctx.db.update(collection, id, data)` | object | Updated document (merges fields) |
| `ctx.db.delete(collection, id)` | void | Deletes the document |

### `ctx.kv`
| Method | Description |
|--------|-------------|
| `ctx.kv.set(key, value, ttlSeconds?)` | Store any value, optional expiry |
| `ctx.kv.get(key)` | Get value or `null` if missing/expired |
| `ctx.kv.delete(key)` | Remove a value |

### `ctx.fetch(url, options?)`
Options: `{ method, headers, body }`. Returns `{ status, body, json() }`. Blocks private IPs.

### `ctx.mail({ to, subject, html?, text? })`
Sends email using the project's mailer config (SMTP or Resend).

### `ctx.publish(channel, data)`
Broadcasts to all WebSocket clients subscribed to that channel. Channel is scoped to the project automatically.

### `ctx.log(...args)`
Writes to the function's execution log, visible in the dashboard.

---

## Limits

| Setting | Value |
|---------|-------|
| Execution timeout | 10s default, max 300s |
| Outbound fetch timeout | 15s |
| Blocked fetch targets | All private/loopback IPs |
| Stored log entries | Last 20 runs per function |
| JS version | ES5.1 + ES6 (no async/await, no require/import) |
