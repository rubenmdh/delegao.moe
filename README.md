# delegao.moe

Personal website, microblog and guestbook in Go, with SQLite or MySQL editing
and static publication. The original public dark theme, animated background,
avatar and content are preserved. Administration uses the same background,
translucent cards, typography and Bootstrap controls as the public website.
GPLv3; see [LICENSE](LICENSE).

For the existing **delegao.moe on an Orange Pi Zero with 512 MB RAM**, use the
Spanish [SQLite migration guide](deploy/sqlite-migration.md). It preserves the
current PHP sites and nginx while importing the external MySQL database into
SQLite. The [alternative MySQL guide](deploy/orange-pi.md) covers native deployment
with a separate MySQL/MariaDB server. Docker is optional, not required on the board.

## Run locally with SQLite

Requires Go 1.26+. SQLite is included in the executable through a pure Go driver;
a separate database server or C compiler is not required. Configure the process
environment; the application does **not** automatically load `.env` files.

PowerShell example:

```powershell
$env:DATABASE_DRIVER = 'sqlite'
$env:DATABASE_DSN = "$PWD/site.db"
$env:PUBLIC_DIR = "$PWD/public"
$env:COOKIE_SECURE = 'false' # Local HTTP only
$env:BUILD_WORKERS = '2'

go run . migrate
go run . create-admin delegao
go run . build
go run .
```

`create-admin` prompts without echoing the password or putting it in the command
line. Passwords must be 12–72 bytes. Visit <http://127.0.0.1:8080/login> to edit
and publish content. When importing existing users, do not create an extra admin.

Build with `go build -trimpath -o delegao .` (`delegao.exe` on Windows). Templates,
images, fonts, styles and migrations are embedded. Generated HTML lives separately
in `PUBLIC_DIR`. The database and public-generation directories must be writable
by the runtime user and kept outside nginx's document root.

## Database choices

- `DATABASE_DRIVER=sqlite`: `DATABASE_DSN` is the path of the local SQLite file.
  Store it on local disk. WAL, a busy timeout and bounded caches support the
  small number of concurrent writes expected from a personal site. Readers and
  a writer can proceed together; SQLite still has one writer at a time.
- `DATABASE_DRIVER=mysql`: `DATABASE_DSN` uses the Go MySQL-driver DSN format,
  e.g. `delegao:password@tcp(127.0.0.1:3306)/delegao`. This remains the default
  when `DATABASE_DRIVER` is omitted, preserving existing installations. MySQL 8
  or MariaDB 10.11+ is supported. For a remote server use verified TLS (`?tls=true`).

SQLite avoids remote round trips and running a local MySQL daemon. It does not
make SD-card latency or durability disappear. Keep backups outside the board.
For concurrent writers across multiple hosts, use the MySQL backend instead.

The `import-mysql` CLI verifies source TLS by default. For a private CA, set
`MYSQL_SOURCE_CA` to its PEM file. An explicitly trusted self-signed source can
instead use `MYSQL_SOURCE_SHA256` with its full leaf-certificate fingerprint.
Pinning checks that exact certificate and its validity period, replacing CA
and hostname checks. Do not combine these options; certificate rotation requires
updating the accepted pin. These settings apply only to the import CLI.

## Hybrid publication

The configured database is the editable source of truth for blog posts. Public
informational pages, published posts, the homepage and paginated archive are
generated before being served. The guestbook, login, previews and administration
stay dynamic. Public reads use files rather than SQL or Markdown conversion.
The original informational pages remain embedded templates rather than
web-editable CMS entries; changing those templates requires recompilation.

Publishing, updating, unpublishing and deleting trigger a complete rebuild with
a bounded worker pool. `BUILD_WORKERS` defaults to 2 and accepts 1–4. These are
generation goroutines inside **one Go server process**, not multiple instances.

A build writes a new generation and switches the manifest when complete. Drafts
and private data are excluded; canonical URLs and pagination switch together.
Database saves and generation are separate stages. If a save succeeds but the
build fails, the panel reports the problem and public HTML returns 503 until a
successful rebuild, including across restarts. The administrator can repair the
problem without discarding the saved content.

Use the panel's rebuild action while running. Stop the service before invoking
`delegao build` from a separate process. The server attempts a rebuild at startup;
if generation fails, the dynamic repair panel remains available and public HTML
returns 503. A database connection failure still prevents startup.

Public HTML has ETags and mandatory revalidation; private forms and sessions use
`Cache-Control: no-store`. Keep nginx proxying **all** routes through Go. Serving
generation directories directly would bypass canonical routing and safeguards.
Back up the source database, not just the reproducible HTML.

## Migration from PHP

The [SQLite migration guide](deploy/sqlite-migration.md) imports a consistent,
read-only snapshot from the external MySQL source into an empty local target.
IDs, timestamps, draft status, tags, guestbook entries and password hashes are
preserved. PHP's stored HTML entities are decoded once. The original MySQL data
is not migrated in place or modified; it remains available for rollback.

If retaining MySQL instead:

1. Back up the database and stop writes from the PHP application.
2. Point `DATABASE_DSN` at a copy, then run `delegao migrate`.
3. Run `delegao build`, verify content/login and switch nginx to Go.

The MySQL migration repairs missing auto-increment IDs and the tags default,
adds the public-post index, and normalizes entities once without changing post
timestamps. Versions are recorded in `schema_migrations`; repeat runs are safe.
MySQL DDL is not transactional. For an in-place MySQL rollback to PHP, restore
the corresponding database backup because PHP expects the previous storage format.

Existing PHP bcrypt (`$2y$`) passwords work; successful login upgrades low-cost
hashes. Sessions are not imported, so sign in again after the switch.

## Routes and editing

The route names in `models.go` keep registration and link generation consistent.
Public posts use `/posts/{id}/{slug}`. `/news`, old `.php` URLs and query-string
post links redirect permanently; old slugs redirect to the current title.
Drafts return 404 publicly. Destructive GET endpoints are unavailable.

- `/admin`: dashboard, editorial counts and publication status.
- `/admin/posts`: search, status filters and creation/editing.
- `/admin/posts/create`, `/admin/posts/{id}/edit`: Markdown editor and preview.
- `/admin/guestbook`: dynamic entry management; emails are admin-only.
- `/guestbook`: public reading/signing; `/login` and POST `/logout` manage sessions.

## Configuration and security

`ADDR` defaults to `127.0.0.1:8080`. The Orange Pi deployment example uses
`127.0.0.1:18080` because another existing site already uses 8080. Keep the Go
port private. Use HTTPS and `COOKIE_SECURE=true` in production. Trust forwarded
IPs only from your own proxy CIDRs; the example nginx proxy overwrites them.

`DB_MAX_OPEN_CONNS=4` and `DB_MAX_IDLE_CONNS=1` bound the application pool.
`GOMEMLIMIT=64MiB` is a Go runtime target, not a hard total-RSS limit. The included
systemd limits are starting values for a 512 MB board; measure the complete machine.

SQL is parameterized and templates escape output. Markdown omits raw HTML and
unsafe URL schemes. Writes require session-bound CSRF protection, including
preview, publishing and logout. The server checks Origin when present, sets
defensive headers, bounds forms and applies request timeouts.

Sessions use random opaque IDs, rotate on login, expire after eight hours
(anonymous forms: 30 minutes), and have HttpOnly / SameSite=Lax cookies. Login,
guestbook and anonymous-session creation are rate-limited. Sessions and limits
live in bounded process memory: restarting signs users out. Run a single instance.
The guestbook retains its honeypot and dish question; update About me if changing
`GUESTBOOK_ANSWER`.

## Backups and verification

For SQLite, install the `sqlite3` command-line utility and use
`deploy/backup-sqlite.sh`. It uses SQLite's online backup operation and checks
the result. **Do not copy only the live `.db` file while WAL is active**; committed
data may still be in its `-wal` companion. For local MySQL, `deploy/backup.sh`
uses a transactional dump. Copy backups off the SD card.

```sh
go test ./...
go vet ./...
```

SQLite integration tests run without an external database. Set
`TEST_DATABASE_DSN` to a MySQL/MariaDB account allowed to create databases for
additional MySQL/import tests. Those tests create/drop their own `delegao_test_*`
databases and do not modify the database named in that DSN. CI also runs the
race detector and ARMv7/ARM64 cross-compilation.

## Optional container

On larger hosts the Dockerfile runs the static executable as an unprivileged
user. Keep the database and publication state on one writable volume. Set
`DATABASE_DRIVER=sqlite` and `DATABASE_DSN=/state/site.db`, or supply a MySQL DSN
reachable from the container. Run `migrate` and initial setup with the same volume
and environment before serving. HTTPS belongs at the reverse proxy.

## Source layout

`app.go` and `admin.go` contain handlers; `models.go` names routes; the store files
contain database backends and import support. Publisher files build/serve HTML.
`security.go` and `proxy.go` implement sessions, CSRF and trusted proxies.
`web/templates/` has shared layouts and `web/static/` the original visual assets
plus native scripts. PHP runtime, installer, Parsedown, jQuery, Bootstrap JS,
Popper and duplicate CSS builds have been removed. Dependencies are pinned in
`go.mod` / `go.sum`. No source files or untracked design experiments are served.
