# CLAUDE.md

Guidance for Claude Code when working in this repository.

## What this is

**Kapture** (Go module `github.com/aribrilliantsyah/kapture`) keeps Kubernetes container logs that would otherwise be lost to rotation or pod replacement. One static binary runs in two modes:

- **agent** — DaemonSet, one per node. Tails `/var/log/containers/*.log`, parses and enriches lines, and stores them in an embedded BadgerDB on the node. Serves an **unauthenticated** internal HTTP API on `:19489`.
- **aggregator** — Deployment. Discovers agents, fans each request out to all of them, merges the replies, and serves the REST API plus the embedded web dashboard on `:19488`. Handles all auth: password plus TOTP 2FA.

The README, `prd.md`, `docs/` and `scripts/` are written in Indonesian. Code, comments and commit messages are in English and use Conventional Commits (`feat(x):`, `fix(deploy):`, `refactor(naming):`).

## Commands

```bash
make build            # CGO_ENABLED=0 → bin/kapture (version/commit injected via ldflags)
make test             # go test ./... -v -race   (-race needs cgo; plain `go test ./...` works without)
go test ./internal/storage/badger -run TestName   # single test
go vet ./...
make testdata         # scripts/generate-testdata.sh → testdata/containers/
make run-agent        # agent on :19489, reads ./testdata/containers, DB in ./testdata/db
make run-aggregator   # aggregator on :19488, static discovery → http://localhost:19489
make docker           # local image kapture:latest
make docker-push VERSION=v1.2.3   # scripts/build-and-push.sh (GitLab registry, creds in scripts/registry.conf, gitignored)
```

The tailer tests take about 15s because they exercise real polling and rotation.

## Layout

```
cmd/kapture/main.go          entry: --mode flag > KAPTURE_MODE; sets slog level, resolves timezone
internal/config              Config structs + DefaultConfig() + LoadFromEnv()
internal/agent/              agent wiring: tailer → batchWriter → store.Write → hub.Publish
  tailer/                    Watcher (fsnotify discovery) + Tailer (poll loop, rate limit) + Reader (per file)
  parser/                    CRI (containerd/CRI-O) and Docker json-file lines; DetectLevel
  enricher/                  filename → ns/pod/container; Resolver (ownerReferences via kube API) with
                             ExtractWorkload (pod-name regex) as fallback
  hub/                       in-memory pub/sub for live tail; never blocks the writer, drops for slow subs
  server/                    agent HTTP API (/api/v1/*, /healthz, NDJSON /api/v1/tail)
internal/aggregator/
  discovery/                 static list, or kubernetes (pod list via API ∪ headless-service DNS, 10s refresh)
  fanout/                    parallel calls to every agent + merge logic (query.go); tail stream fan-in (tail.go)
  fanout/backup.go           backup archive (tar.gz of per-agent Badger backups) and restore routing
  handler/                   REST routes; auth.go (middleware + sign-in steps), users.go (users + profile),
                             storage.go (backup/restore), live.go (WebSocket tail + recap)
internal/auth                Manager: users + roles, bcrypt, TOTP, step-based sign-in, recovery questions,
                             HMAC-signed session tokens, login throttling
internal/storage/store.go    Store interface (the agent API mirrors it 1:1)
internal/storage/badger      Badger implementation: keys.go (key schema), badger.go (write/delete/GC/migrate),
                             query.go (scan/Query/Volume), index.go (in-memory rollups + catalog)
internal/logfilter           compiles a QueryRequest into a Filter (MatchMeta on key fields, MatchText on message)
internal/search              dashboard query syntax: AND, OR, -NOT, "phrase", field:value, /regex/
internal/kube                tiny read-only in-cluster API client (no client-go, keeps the binary small)
internal/model               LogEntry, QueryRequest (ParseQuery/Values round-trip), results, NormalizeLevel
internal/version             Version/Commit, set via -ldflags -X
web/                         go:embed of web/static; dashboard is plain ES modules, NO build step
deploy/                      numbered manifests, apply with `kubectl apply -f deploy/`
```

## Architecture notes and invariants

### Configuration is env-only
`config.example.yaml` is **documentation only**. No Go code parses YAML. Settings come from `DefaultConfig()` and are overridden by `LoadFromEnv()`, which reads `KAPTURE_*` and falls back to the legacy `LOG_CATCHER_*`. A new setting needs a struct field, a default, and a `getEnv(...)` branch in [internal/config/config.go](internal/config/config.go).

Some fields are declared but not wired up: `Exclude.Labels`, `API.GRPCPort` (reserved) and `Collector.MaxGoroutines`. Compression, GC interval and collector tuning have no env override, so only their defaults apply.

`KAPTURE_PASSWORD` being set forces auth on. `KAPTURE_TIMEZONE` (IANA name, falls back to `TZ`, then UTC) decides how logs split into days. The binary embeds `time/tzdata`.

### Ingestion pipeline (agent)
- `/var/log/containers/*.log` are symlinks into `/var/log/pods`, and inotify never reports writes to symlink targets. Readers are therefore **polled every 1s**. fsnotify only speeds up file discovery, and a rescan every 30s catches missed events.
- `Reader` advances its offset only past complete lines. It handles rotation (inode change: drain the old file, then open the new one from 0) and copytruncate (size < offset). It persists `(offset, inode)` under `_offset:<path>`.
- CRI `P` partial lines are joined up to 256 KiB. Continuation lines (leading space or tab, `Caused by:`, `...`) within 2s on the same stream are appended to the previous entry, which handles stack traces.
- `Seq` = byte offset of the line + 1. Re-reading a file therefore **overwrites the same key instead of duplicating it**. Keep this property.
- The rate limiter (`RateLimit` lines/s per node) makes lines wait and never drops them.
- Workload resolution: the pod's controller owner is followed to Deployment or CronJob and cached permanently. On API failure it falls back to pod-name regex for 2 minutes, with a 1-minute API backoff. RBAC for this is in `deploy/01-rbac.yaml`.

### Storage (Badger) — see [keys.go](internal/storage/badger/keys.go)
- Log key: `<date>:<ts hex16>:<ns>:<workload>:<wtype>:<pod>:<container>:<level>:<seq hex16>`. Value: 1 stream byte (`o`/`e`/`-`) followed by the message.
- Keys sort chronologically across all pods. Filters that use only key fields never read values. Deleting a whole day is a cheap `DropPrefix("<date>:")`.
- `<date>` is the day in the **configured timezone**, not UTC. Comments that say "UTC" in `keys.go`, `model/log.go` and `LogEntry.Date` are stale: `Store.Write` recomputes the date with `dateOf(ts, loc)`.
- `:` inside names is replaced by `_` (`clean`). Log keys start with a digit and internal keys with `_` (`_offset:`, `_meta:`, `_agg:`, `_cat:`). `dataLo="0"` and `dataHi="_"` bound log scans.
- `index.go` keeps daily per-workload rollups (`_agg:`) and a container catalog (`_cat:`) in memory and flushes them every 5s. Dashboards, `Dates()`, `Namespaces()`, storage stats and recap read the index, not a log scan. Any change to write or delete paths must keep the index consistent (`observeLocked`, `removeDays`).
- Write paths take `s.mu.RLock`. `DropPrefix` takes the write lock so writes wait instead of hitting `ErrBlockedWrites`.
- Scans have a budget of 3M keys or 5s. Past that the result carries `partial=true` plus a cursor.
- GC loop: value-log GC, then retention (drop whole days older than the cutoff), then the disk cap (drop the oldest day while above `MaxDisk`, always keeping one day).
- Backup/restore ([backup.go](internal/storage/badger/backup.go)): `Backup` streams only log keys (Badger backup format, `ChooseKey` filter, optional date range). No `_offset:`, `_meta:` or index keys: a restore must never move another node's read positions. `Restore` parses the frames itself instead of `db.Load`, so lines get fresh versions, are re-dated to the local zone, follow local retention and overwrite duplicates. It then calls `reindex()`, which rebuilds the index under the write lock.
- The aggregator wraps agent backups as `nodes/<node>/NNNNNN.badger` tar parts of 8 MiB each (a tar header needs the size up front) plus `manifest.json`. Agents report the line count or an error in HTTP trailers. On restore, each node goes to the agent with the same node name, otherwise to the first reachable agent.
- **Schema changes**: bump `schemaVersion` and add a case to `migrate()`. Versions not listed there get `DropAll` (logs **and** offsets are wiped and files re-ingested). A timezone change triggers a one-time `rekey()` plus index rebuild.

### Query and merge (aggregator)
- `/api/v1/*` routes on the aggregator mirror the agent's routes. `model.ParseQuery` and `QueryRequest.Values()` must stay symmetric, because the aggregator forwards requests by re-encoding them.
- Pagination cursor = unix-nano timestamp, exclusive in the sort direction (default `desc`). `fanout.QueryLogs` clips the merged page at the furthest agent `NextCursor` so the next page cannot skip entries. Read the comment there before touching it.
- Errors: when every agent rejects a request with a 4xx (e.g. a bad regex), that status is returned. Otherwise unreachable agents show up in `errors[]` and the reply is still 200.
- Live tail: the browser opens a WebSocket at `/api/v1/tail` on the aggregator. The aggregator opens an NDJSON stream to each agent's `/api/v1/tail`, re-discovers agents every 10s, batches every 250ms, and caps pending entries at 2000.
- Export (`/api/v1/logs/export?format=csv|json`) pages through results, capped at `max_results`.

### Auth
- State lives in `auth.json` (`KAPTURE_AUTH_FILE`, default `/data/kapture/auth.json`, falls back to `./data/auth.json`), file version 2 holds a user list. A version 1 single-admin file is migrated on load; old sessions then become invalid. First run redirects to `/setup` for the first admin: username, password, recovery question and TOTP enrollment.
- Password policy (`checkPassword`, mirrored in `ui.js` `passwordProblem`/`passwordRules`): at least 8 characters with a lowercase letter, an uppercase letter, a digit and a symbol, at most 72 bytes. It applies wherever a password is set; existing passwords are not re-checked. `randomPassword()` always produces a compliant temporary password.
- Roles: `admin` and `operator`. The only difference is user management: `Wrap` returns 403 for `/api/v1/users*` to non-admins, and `IdentityOf(r)` gives handlers the user. The last admin cannot be deleted or demoted. With auth disabled, every request is `auth.Anonymous` (admin).
- Sign-in is a chain of steps (`StepTOTP`, `StepPassword`, `StepEnroll`) tied to a 5-minute temp token. Admin-created or admin-reset accounts must change their password, and accounts without a TOTP secret enroll one. An expired or unknown temp token returns `401 {"expired": true}`, and the login page then goes back to the password form with the message.
- An admin can set another admin's recovery question with `PUT /api/v1/users/{id}/recovery`; the own question goes through `/profile/recovery`, which requires the password.
- Recovery is for admins only (`/auth/recover/*`): the answer **plus** one more factor, the TOTP code to reset the password or the password to reset 2FA. The answer alone must never be enough for both. Answers are normalized (lower case, collapsed spaces) and bcrypt-hashed. Question ids are fixed in `recovery.go`.
- Session token = `base64url(json{u: userID, g: SessionGen, ...}).base64url(HMAC-SHA256)`, signed with a key persisted in `auth.json`, so sessions survive restarts. Bumping a user's `SessionGen` (password change, admin reset) ends all their sessions. Tokens last 30 days, are re-issued once older than 24h, and travel in the HttpOnly cookie `kapture_session` or an `Authorization: Bearer` header. `POST /api/v1/auth/login` does password and TOTP in one call for scripts.
- TOTP replay is blocked per user and secret (`lastOTP`). After 5 failures within 10 minutes, the client (first `X-Forwarded-For` hop) and the username (`user:<name>`) are locked for 5 minutes.
- `Handler.Wrap` exempts only `/healthz`, `/api/v1/auth/*` and the paths in `isAsset()` (`/js/`, `/styles/`, `/fonts/`, `/favicon.png`, `/appicon.png`). **Any new public static path must be added to `isAsset`**, or it redirects to login.

### Dashboard (web/static)
- Plain ES modules with no bundler, npm or framework. Entry points: `index.html` → `js/app.js` (hash router, views in `js/views/*.js`) and `auth.html` → `js/auth-page.js`. Lucide icons are inlined as SVG symbols in both HTML files, so a new `icon('x')` needs a `<symbol id="i-x">` there.
- Ctrl+K opens the command palette (`js/palette.js`), which jumps to pages, workloads and namespace scopes. Log search lives only in the Explorer and Compare toolbars (`[data-log-search]`, focused with `/`). The namespace scope shows as a topbar chip.
- Log messages may contain ANSI color codes. Render them with `renderAnsi(text, fmt)` from `js/ansi.js`, and use `stripAnsi` for one-line previews and copying. Colors come from the `--ansi-0..15` CSS variables, set per theme and for `.terminal`. The agent strips ANSI codes before detecting the level (`parser.DetectLevel`).
- `js/views/about.js` is the About page (author, stack, why the tool exists, AI credits). Author details and the stack list live in that file only.
- Charts live in `js/chart.js`: `volumeChart` (stacked levels), `lineChart`, `sparkline`, `shareBar` (part-to-whole), `statTile`. They follow the dataviz rules: niceTicks gridlines, bars at most 24px, values-first tooltips, keyboard arrows, a Chart/Table toggle, and text in text colors (only marks use `--lv-*`). `perDay`/`sumCounts` in `state.js` turn recap rollups into daily series. The dashboard compares the chosen period (`#/dashboard?days=7|14|30`) with the one before.
- Pitfall: `el.replaceChildren(...)` and `append(...)` do not flatten arrays; an array argument renders as "[object HTMLDivElement]". Spread it (`...list`) or wrap it in `h()`, which flattens.
- Font: JetBrains Mono Nerd Font Mono (Regular and Bold woff2, SIL OFL) in `web/static/fonts/`, under the family name `"Kapture Mono"`.
- `js/api.js` wraps `fetch('/api/v1'+path)`, and a 401 redirects to `/login?next=...`. `js/state.js` holds the catalog, agents, namespace scope and pins, plus `localStorage` keys prefixed `kapture_`.
- The files are compiled into the binary via `go:embed`, so **rebuild or re-run after editing any frontend file**. HTML pages are served `no-cache`.

## Conventions
- Only `log/slog` for logging, and only the standard library `net/http` mux with Go 1.22+ method patterns (`"GET /api/v1/logs"`). Keep dependencies minimal: the direct deps are badger, fsnotify, coder/websocket, go-qrcode and x/crypto. Do not add client-go.
- The agent's `reply()` maps errors to 400. The aggregator's `reply()` maps agent `StatusError` codes and falls back to 502.
- Comments explain *why* and are short. Match that density.
- Default ports: aggregator 19488, agent 19489. Kubernetes namespace `kapture`, agent labels `app=kapture,role=agent`, headless service `kapture-agents`.
- The Docker image also symlinks `logcatcher` → `kapture` for backward compatibility (the old `cmd/logcatcher` was removed).
