# delegao.moe

Personal website, microblog and guestbook in Go, with MySQL editing and static
publication. The original public dark theme, animated background, avatar and
content are preserved. The administration area has its own responsive layout.
GPLv3; see [LICENSE](LICENSE).

For **delegao.moe on an Orange Pi Zero with 512 MB RAM**, follow the Spanish
[deployment guide](deploy/orange-pi.md). It includes ARM builds, MariaDB, systemd,
nginx, TLS, backups and updates. Native deployment is recommended for this board.

## Run locally

Requires Go 1.26+ and MySQL 8 / MariaDB 10.11+. Configure the process environment
using `.env.example` as a reference; the application does **not** load `.env`
files itself. PowerShell example:

```powershell
$env:DATABASE_DSN = 'delegao:change-me@tcp(127.0.0.1:3306)/delegao'
$env:PUBLIC_DIR = "$PWD/public"
$env:COOKIE_SECURE = 'false' # Local HTTP only
$env:BUILD_WORKERS = '2'
go run . migrate
go run . create-admin delegao
go run . build
go run .
```

Create the database first. `create-admin` prompts without echoing the password
or putting it in the command line. Passwords must be 12–72 bytes. Visit
<http://127.0.0.1:8080/login> to edit and publish content.

Build an executable with `go build -trimpath -o delegao .` (`delegao.exe` on
Windows). Templates, images, fonts, styles and migrations are embedded in the
binary. Published HTML is generated separately in `PUBLIC_DIR`, which must be
writable by the runtime user. Do not put secrets or source files in that folder.

## Hybrid publication

MySQL is the editable source of truth for blog posts. Public informational pages, published
posts, the homepage and paginated archive are generated before being served.
The guestbook, login, previews and administration stay dynamic. Public reads
use generated files rather than SQL or Markdown conversion on every visit.
The original informational pages are embedded templates, not web-editable CMS
entries; changes to those templates require recompilation and a rebuild.

Publishing, updating, unpublishing and deleting content trigger a complete
rebuild with a bounded worker pool. `BUILD_WORKERS` defaults to 2 and accepts
1–4; start with 1 or 2 on small boards. This is one Go server process with
worker goroutines, not several web-server instances.

A build writes a new generation and changes the current manifest only when
complete. Drafts and private data are excluded. Canonical URLs and pagination
are switched together; removed entries and obsolete output disappear from the
active generation. MySQL saves and publication are separate stages: if the
save succeeds but generation fails, the panel reports it and public HTML is
blocked with 503 until a successful rebuild, including across restarts. The
administrator can repair the problem and rebuild without discarding the save.

Use the panel's rebuild action while running. For a command-line rebuild, stop
the service before `delegao build`: only one process may control `PUBLIC_DIR`.
The server attempts a fresh rebuild at startup. If generation fails, the
dynamic repair panel stays available while public HTML returns 503. A database
connection failure still prevents startup. The CLI `build` command returns a
nonzero status if it cannot finish.
Public HTML has ETags and mandatory revalidation; private forms and sessions
use `Cache-Control: no-store`. Keep nginx proxying **all** routes through Go:
serving generation directories directly would bypass canonical routes and
failed-publication safeguards. Do not add a shared proxy cache for admin or
CSRF forms. Back up MySQL, not only generated HTML.

## Migration from PHP

1. Back up the database and stop writes from the PHP application.
2. Point `DATABASE_DSN` at a copy of that database, then run `delegao migrate`.
3. Run `delegao build`, verify the content and login, then repeat with the
   deployment database and switch the proxy to Go.

Existing `news`, `guestbook` and `users` IDs and PHP bcrypt (`$2y$`) passwords
are preserved. The migration repairs missing auto-increment IDs and the tags
default, adds the public-post index, and decodes PHP's stored HTML entities once
without changing post timestamps. Migration versions are recorded in
`schema_migrations`; repeated runs are safe. MySQL schema changes are not
transactional. For rollback to PHP, restore the backup and the `legacy` branch
together because PHP expects the previous text storage format.

## Routes and editing

The route names in `models.go` keep registration and link generation consistent.
Public posts use `/posts/{id}/{slug}`. `/news`, old `.php` URLs and query-string
post links redirect permanently; old slugs redirect to the current title.
Drafts return 404 publicly. Destructive GET endpoints are unavailable.

- `/admin`: dashboard, editorial counts and publication status.
- `/admin/posts`: posts, search/filter controls and creation/editing.
- `/admin/posts/create`, `/admin/posts/{id}/edit`: Markdown editor and preview.
- `/admin/guestbook`: dynamic entry management; emails are admin-only.
- `/guestbook`: public reading and signing; `/login` and POST `/logout` manage sessions.

## Configuration and security

`ADDR` defaults to loopback `127.0.0.1:8080`. Use HTTPS and
`COOKIE_SECURE=true` in production. Set `TRUSTED_PROXIES` only to your own proxy
CIDRs. The included nginx configuration overwrites forwarded IPs, preserves
Host and keeps the Go port private. For a remote database configure verified
TLS in the DSN, e.g. `?tls=true`; the board guide uses a local Unix socket.

`DB_MAX_OPEN_CONNS=4` and `DB_MAX_IDLE_CONNS=1` bound the application pool.
The open-connection setting accepts 2–16; migrations need a connection in
addition to the one that holds their database lock.
`GOMEMLIMIT=64MiB` is a Go runtime target, not a hard total-memory limit.
The deployment guide provides a cautious starting profile for 512 MB systems;
measure the complete machine under your workload before increasing concurrency.

SQL is parameterized; templates escape output. Markdown omits raw HTML and
unsafe URL schemes. All writes require session-bound CSRF protection, including
preview, publishing and logout. The server also checks Origin when present,
sets defensive headers, bounds forms, and applies request timeouts.

Sessions use random opaque IDs, rotate on login, expire after eight hours
(anonymous forms: 30 minutes), and use HttpOnly / SameSite=Lax cookies.
Login attempts, guestbook submissions and anonymous-session creation are
rate-limited. Restarting signs users out: sessions and rate limits are in
bounded process memory, so run a **single server instance**. The guestbook
retains its honeypot and dish question. If `GUESTBOOK_ANSWER` changes, update
the About me content to match.

## Verification

```sh
go test ./...
go vet ./...
```

Set `TEST_DATABASE_DSN` to a MySQL/MariaDB account allowed to create databases
and run `go test -count=1 ./...` for real database integration tests. Each run
creates and drops its own `delegao_test_*` database; it does not modify the
database named in that DSN. CI runs integration tests, the race detector and
ARMv7/ARM64 cross-compilation.

## Optional container

The Orange Pi guide uses systemd instead. On larger hosts the Dockerfile runs
a static executable as an unprivileged user. Keep `/state` on a writable volume
and pass a DSN reachable from the container (not host loopback):

```sh
docker build -t delegao .
docker run --rm --read-only --mount type=volume,src=delegao-state,dst=/state \
  --env-file .env -e PUBLIC_DIR=/state/public -e ADDR=0.0.0.0:8080 \
  -p 127.0.0.1:8080:8080 delegao
```

Run the migration and initial build with the same environment and volume before
starting the service. HTTPS still belongs at the reverse proxy.

## Source layout

`app.go` and `admin.go` contain handlers; `models.go` names routes; `store.go`
contains persistence; the publisher files build and serve generated HTML.
`security.go` and `proxy.go` implement sessions, CSRF and trusted proxies.
`web/templates/` contains shared layouts, and `web/static/` contains the original
visual assets plus small native scripts. The PHP runtime, installer, Parsedown,
jQuery, Bootstrap JS, Popper and duplicate CSS builds have been removed.

Dependencies are pinned in `go.mod` / `go.sum`. Only Bootstrap 4 CSS and Font
Awesome CSS/fonts remain from the original frontend libraries. No source files
or untracked design experiments are served by the application.
