# delegao.moe

Personal website, microblog and guestbook, rewritten in Go. The original dark
Bootstrap theme, animated background, avatar and page content are preserved.
GPLv3; see [LICENSE](LICENSE).

## Run locally

Requires Go 1.26 or later and MySQL 8 / MariaDB 10.11 or later. Go serves the
website directly; PHP, Apache rewrite rules and Node.js are unnecessary.

1. Create a UTF-8 database and a database user.
2. Set the environment variables listed in `.env.example`. The application reads
   the process environment; it does **not** automatically load `.env` files.
3. Run `go run . migrate`, then `go run . create-admin YOUR_USERNAME`.
   The latter prompts for a password without echoing it or putting it in the
   command line. Passwords must be 12–72 bytes.
4. Run `go run .` and open <http://127.0.0.1:8080>.

PowerShell example (replace the database credentials):

```powershell
$env:DATABASE_DSN = 'delegao:change-me@tcp(127.0.0.1:3306)/delegao'
$env:COOKIE_SECURE = 'false' # Local HTTP only
go run . migrate
go run . create-admin delegao
go run .
```

Build a standalone executable with `go build -trimpath -o delegao .` (use
`delegao.exe` on Windows). Templates, images, fonts, styles and migrations are
embedded in the executable. No working-directory files are served.

## Migrating the PHP installation

1. Back up the existing database and stop writes from the PHP application.
2. Point `DATABASE_DSN` at a **copy** of that database and run `delegao migrate`.
3. Verify the content and login, then repeat the migration on the deployment
   database and switch the reverse proxy to the Go server.

Existing `news`, `guestbook` and `users` tables and IDs are preserved. The
migration repairs missing auto-increment IDs and the tags default, adds an index
for public posts, and decodes the HTML entities previously stored by PHP.
The decoding runs once, transactionally, and does not change post timestamps.
Migration versions are recorded in `schema_migrations`; repeated runs are safe.
Schema changes use MySQL DDL and are not transactional. Run migrations as a
separate deployment step before starting the server.

Existing PHP bcrypt (`$2y$`) passwords work. Successful logins upgrade hashes
with a cost below 12. PHP sessions are not imported; sign in again.
The PHP installer and database endpoints have been removed. For rollback,
restore the backup and the `legacy` branch together, because PHP expects its
previous text storage format.

## Routes

Routes and names are centralized in `models.go`; templates call `route` to
generate links. `app.go` binds HTTP methods and authentication middleware using
Go's standard `http.ServeMux` path parameters.

| Method | URL | Purpose |
| --- | --- | --- |
| GET | `/` | Latest published post |
| GET | `/posts` | Paginated published posts |
| GET | `/posts/{id}/{slug}` | Published post, e.g. `/posts/42/hello-world` |
| GET | `/about-me`, `/projects`, `/music`, `/games`, `/contact`, `/keys`, `/donate` | Original pages |
| GET, POST | `/guestbook` | Read / sign the guestbook |
| GET, POST | `/login` | Sign in |
| POST | `/logout` | Sign out |
| GET | `/admin` | Dashboard |
| GET, POST | `/admin/posts` | Manage / create posts |
| GET | `/admin/posts/create` | New post form |
| GET | `/admin/posts/{id}/edit` | Edit form |
| POST | `/admin/posts/{id}` | Update a post |
| POST | `/admin/posts/{id}/delete` | Delete a post |
| GET | `/admin/guestbook` | Manage entries, including email addresses |
| POST | `/admin/guestbook/{id}/delete` | Delete an entry |

Old page URLs, `/news`, `/index.php?id=42`, `/index?id=42`, `/?id=42`
and `/posts/42` redirect permanently to the new URLs. An old or incorrect post
slug redirects to the current title; its ID remains stable. Drafts return 404
publicly. Old destructive GET endpoints are deliberately unavailable.

## Security and deployment

- Parameterized SQL; validated and bounded forms; escaped HTML templates.
- Markdown uses Goldmark's safe defaults. Raw HTML and unsafe link schemes are
  disabled. Excerpts are plain text rather than truncated HTML.
- All writes, including login and logout, require a session-bound CSRF token.
  Forms also check the Origin header when present. GET requests cannot delete.
- Random opaque sessions, rotated on login and invalidated on logout, with
  HttpOnly, SameSite=Lax cookies. `COOKIE_SECURE` defaults to `true`; use HTTPS
  in production. Sessions expire after eight hours (anonymous forms: 30 minutes).
- Rate limits for login (10 attempts / 15 minutes), guestbook submissions
  (5 / 15 minutes) and new anonymous sessions (60 / 15 minutes), plus a guestbook
  honeypot and the original dish question.
- Guestbook emails are visible only to administrators.
- CSP, anti-framing and MIME-sniffing headers; bounded requests and server
  timeouts. Assets are served from an embedded allowlist; source code, database
  configuration and the design experiments are not public files.

Set `ADDR` to a loopback address behind an HTTPS reverse proxy and preserve the
public `Host` header. If using a container, set `ADDR=0.0.0.0:8080` and expose it
only through your proxy. For a remote database, configure verified TLS in the
MySQL DSN, for example `?tls=true`.

Forwarded IP headers are ignored by default. Set `TRUSTED_PROXIES` to the CIDR
ranges of **your** proxies, e.g. `127.0.0.1/32,::1/128`; then the server walks
`X-Forwarded-For` from the trusted proxy towards the client for rate limiting.
The proxy must append or overwrite that header and forward the original Host.
Keep `COOKIE_SECURE=true` when the proxy terminates HTTPS.

Sessions and rate limits live in bounded process memory. Restarting the process
signs everyone out. Run one instance; multiple instances require shared session
and rate-limit storage. The dish question remains a lightweight spam deterrent.
If `GUESTBOOK_ANSWER` changes, update the answer in the About me template too.

The Dockerfile builds a static binary and runs it as an unprivileged user:

```sh
docker build -t delegao .
docker run --rm --read-only -p 127.0.0.1:8080:8080 \
  --env-file .env -e ADDR=0.0.0.0:8080 delegao
```

## Verification

```sh
go test ./...
go vet ./...
```

For real database integration tests, set `TEST_DATABASE_DSN` to a MySQL/MariaDB
account allowed to create databases, then run `go test -count=1 ./...`.
Tests create and drop uniquely named `delegao_test_*` databases; they do not
modify the database named in the DSN. These tests cover the legacy schema,
idempotent migrations, PHP bcrypt login, draft isolation, CRUD, canonical
redirects, guestbook privacy, XSS, CSRF and pagination. CI runs them with MariaDB.

## Layout and dependencies

- `main.go`: configuration, CLI commands and server lifecycle.
- `app.go`, `models.go`: handlers, named routes, models and safe rendering.
- `security.go`, `proxy.go`: sessions, CSRF, rate limits and trusted proxies.
- `store.go`, `migrations/`: SQL persistence and migration from PHP.
- `web/templates/`: shared layouts and the original page content.
- `web/static/`: original visual assets and a small native JavaScript file.

Only Bootstrap 4's CSS and Font Awesome's CSS/fonts remain from the original
frontend libraries. jQuery, Bootstrap JS, Popper, duplicate CSS builds, Parsedown,
the PHP installer and all PHP runtime files are removed. Go dependencies are
pinned in `go.mod` / `go.sum`: the MySQL driver, Goldmark, and Go's crypto, text
and terminal packages.
