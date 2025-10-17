# tenvy-server Agents Guide

This document is a "README for machines" – use it to onboard autonomous contributors to the server-side SvelteKit app. It summarizes how the backend is wired, where major features live, and which guardrails protect the project.

## Stack at a glance
- **Framework**: SvelteKit 2 on Svelte 5 (Runes mode) with Vite.
- **Language**: TypeScript throughout (`type = "module"`).
- **UI**: Tailwind CSS v4, shadcn-svelte (Bits UI), lucide icons.
- **Server runtime**: Bun (preferred) or Node 20+; persistence via `better-sqlite3` driven by Drizzle ORM.
- **Shared contracts**: Re-use `shared/` TypeScript definitions (imported with `$lib/../../../../shared/...`).
- **Testing**: Vitest (unit), Playwright (e2e), Svelte Testing Library for component specs.

Use `bun` commands unless explicitly noted otherwise – the lockfile is `bun.lock`.

## Project layout
```
tenvy-server/
├── src/
│   ├── hooks.server.ts       # Global auth/session middleware
│   ├── hooks.ts              # Localizes routes via Paraglide
│   ├── lib/
│   │   ├── assets/           # Static images/icons packaged by Vite
│   │   ├── components/       # UI building blocks (Runes-ready)
│   │   ├── data/             # Static datasets (e.g., map/topology data)
│   │   ├── hooks/            # Client-side helper hooks/stores
│   │   ├── server/
│   │   │   ├── auth.ts & auth/         # Session, voucher, recovery helpers
│   │   │   ├── db/                     # Drizzle schema & SQLite bootstrap
│   │   │   ├── rat/                    # Agent registry, command relays, QUIC
│   │   │   ├── recovery/               # Offline recovery archive storage
│   │   │   ├── task-manager/           # Host process orchestration helpers
│   │   │   └── rate-limiters.ts        # Rate limiter wrappers
│   │   ├── stores/          # Writable/derived stores for UI state
│   │   ├── types/           # UI-specific typing glue
│   │   ├── utils(.ts)/      # Shared helpers (`cn`, etc.)
│   │   └── validation/      # Zod schemas for inbound payloads
│   └── routes/
│       ├── +layout.*, +page.*     # Marketing/unauthenticated shell
│       ├── login/, redeem/        # Auth onboarding & passkey bootstrap
│       ├── api/                   # REST-ish JSON endpoints
│       └── (app)/                 # Authenticated dashboard surface
├── tests/                  # Vitest server-side/unit specs
├── e2e/ & playwright.config.ts   # Playwright suites
├── static/                 # Files served verbatim
└── drizzle.config.ts       # CLI entry for schema migrations
```

### Routing strategy
- `src/hooks.server.ts` sequences Paraglide locale resolution and session attachment for **every** request. Auth state is exposed through `event.locals.user` & `event.locals.session`.
- `(app)/+layout.server.ts` protects the authenticated area. Unauthed users are redirected to `/login` or `/redeem` depending on passkey status.
- Public login/redeem flows keep logic in `+page.server.ts` modules so that form actions can call into `$lib/server` helpers.
- API routes under `src/routes/api/**` are thin JSON wrappers that delegate to `$lib/server` modules. Keep business logic out of `+server.ts` files.

## Core server modules

### Authentication & onboarding (`src/lib/server/auth*`)
- Sessions are opaque tokens stored in SQLite (`session` table). Tokens are hashed via SHA-256 before persistence (`generateSessionToken`, `createSession`).
- Voucher enforcement happens in `validateSessionToken`. Sessions auto-renew for long-lived cookies but terminate if voucher is inactive/expired.
- `auth/recovery.ts` issues recovery codes (10 default) with uppercase alphanumeric groups and stores hashes in `recovery_code`.
- `recovery/` namespace owns disk-based archive upload metadata (`storage.ts`) with strong checksum verification and tamper checks. Errors bubble as typed classes (`RecoveryArchiveIntegrityError`, etc.).
- Rate limiting for voucher redeem / WebAuthn flows lives in `lib/server/rate-limiters.ts` – reuse `limitVoucherRedeem` & `limitWebAuthn` instead of instantiating new limiters.

### Persistence (`src/lib/server/db`)
- `db/index.ts` initializes a single `better-sqlite3` client, turning on foreign keys and lazily adding new columns via `ensureColumn`. Tables are created idempotently on startup.
- `db/schema.ts` defines all tables using Drizzle. Keep schema changes mirrored in migrations (`drizzle-kit`).
- Set `DATABASE_URL` (file path or `file:` URI). Tests may point to an in-memory sqlite file.

### Agent registry & RAT stack (`src/lib/server/rat`)
- `store.ts` is the heart of the controller: it manages agent metadata, websockets, command queues, shared notes, and persistence. An eagerly constructed singleton (`export const registry`) is imported by routes and websocket handlers.
  - Persistent storage defaults to `var/registry/clients.json` but can be moved via `TENVY_AGENT_REGISTRY_PATH`.
  - Agent authentication expects `TENVY_SHARED_SECRET` (if set) to match the incoming registration token.
  - Shared notes, pending commands, and result histories are debounced and written atomically. Use helper methods (e.g., `registerAgent`, `enqueueCommand`, `syncNotes`) instead of mutating maps.
  - Public responses should wrap thrown errors via `RegistryError` so HTTP handlers can surface the correct status codes.
- Other files map to discrete capabilities:
  - `client-chat.ts` – in-memory chat session management.
  - `clipboard.ts` & `.test.ts` – clipboard sync payload validation.
  - `file-manager.ts` – archive upload/download orchestration using JSZip.
  - `audio.ts`, `remote-desktop.ts`, `remote-desktop-input.ts` – WebRTC & QUIC signaling helpers. QUIC listener respects `TENVY_QUIC_INPUT_*` env vars (`ADDRESS`, `PORT`, `KEY`, `CERT`, `DISABLED`, `AUTOSTART`).
  - `session.test.ts`, `file-manager.test.ts`, etc. document expected command lifecycles – read them before changing command payloads.

### Build service (`src/routes/api/build` & `src/lib/validation/build-schema.ts`)
- Normalizes payloads from the builder UI before they reach the Go build pipeline.
- Always validate inbound data against `buildRequestSchema` and respond with the `BuildResponse` contract. `normalizer.ts` handles defaults and normalization.

### Recovery workflows (`src/lib/server/recovery`)
- Archives are stored per-agent under `var/recovery/<agentId>/`. Metadata & manifests are JSON; actual binaries are `.zip` files.
- `storage.ts` writes atomically via temp files and uses SHA-256 to guard against corruption. On conflicts, it emits `RecoveryArchiveConflictError` referencing the existing archive ID.
- `validation.ts` contains zod validators to normalize manifest & metadata.

## Environment variables
| Variable | Purpose |
| --- | --- |
| `DATABASE_URL` | Required. SQLite connection string/path for auth & voucher tables. |
| `TENVY_SHARED_SECRET` | Optional shared token for agent registration requests. |
| `TENVY_AGENT_REGISTRY_PATH` | Override location of `clients.json` registry snapshot. |
| `TENVY_RECOVERY_DIR` | Override root directory for recovery archives. |
| `TENVY_QUIC_INPUT_ADDRESS` / `PORT` / `KEY` / `CERT` / `DISABLED` / `AUTOSTART` | Configure the QUIC input listener for remote desktop control. |

## Working with shared packages
- Import shared types from the monorepo (`shared/types/...`). Keep TS source-of-truth there so both the server and Go agent stay aligned.
- Avoid duplicating schemas – prefer `zod` adapters located under `src/lib/validation` when accepting new payloads.
- When updating shared types, run the relevant generators if any (check `shared/` docs) and adjust server-side validators/tests in the same commit.

## Coding conventions
- **Formatting/Linting**: run `bun format` before committing. `bun lint` wraps Prettier + ESLint; `bun check` runs `svelte-check`.
- **Components**: follow Svelte 5 runes patterns. Compose UI via `src/lib/components` and keep layout-specific logic in route components.
- **Server utilities**: keep side-effectful singletons (registry, rate limiters) in `$lib/server`. Inject dependencies through parameters when possible to ease testing.
- **Error handling**: throw typed errors (e.g., `RegistryError`, `RecoveryArchive*Error`) and let routes translate them to HTTP responses.
- **Persistence**: use provided helpers (`writeFileAtomic`, `ensureParentDirectory`) for filesystem writes to avoid partial state.

## Testing & quality gates
- `bun test` runs Vitest suites (`tests/` + `src/**/*.spec.ts`). Use `describe/it` semantics.
- `bun run test:e2e` launches Playwright (requires browsers installed). Skip in CI if the environment lacks Playwright.
- Use `bun check` before committing to catch TypeScript + Svelte diagnostics.
- Add unit specs alongside the module (`*.test.ts`) when altering server logic, especially inside `lib/server/rat`.

## Local development workflow
1. Install deps: `bun install`.
2. Set `DATABASE_URL` (e.g., `file:var/dev.sqlite`).
3. Launch dev server: `bun run dev` (defaults to `localhost:5173`).
4. For Playwright tests, run `npx playwright install` once.
5. Drizzle migrations: `bun run db:generate` (generate SQL), `bun run db:push` to sync.

## Extending the system
- **New API endpoint**: create `src/routes/api/<feature>/+server.ts`, validate input with Zod in `src/lib/validation`, and dispatch to a `$lib/server` module. Export types in `shared/` if the Go agent needs them.
- **Modifying agent commands**: adjust types under `shared/types/messages`, update `AgentRegistry` command handlers & persistence serialization, then refresh unit tests.
- **Adding dashboard pages**: create a new folder under `src/routes/(app)/` and wire navigation state via `NavKey` in `(app)/+layout.ts`.
- **Updating recovery flow**: change validators in `lib/server/recovery/validation.ts`, keep `storage.ts` atomic operations intact, and document any new metadata fields in the shared types.

## Gotchas
- `AgentRegistry` loads state during construction; avoid importing modules that mutate the singleton before env vars are configured.
- WebAuthn/voucher flows are rate-limited per identifier; bypassing `limitVoucherRedeem` or `limitWebAuthn` can expose brute-force risks.
- QUIC input will auto-start unless `TENVY_QUIC_INPUT_AUTOSTART` is set to `'0'`. Disable in tests to avoid binding sockets.
- Recovery archive writes are crash-safe but not concurrent-safe – queue writes per agent if you add async workers.
- When using Node instead of Bun, ensure optional dependencies (`@koush/wrtc`) are available; headless environments may need stubs/mocks for unit tests.

## Related workspaces
- `shared/` contains TS/Go shared logic. Update this first when changing data contracts.
- `tenvy-client/` (Go agent) consumes outputs from the build API and registry; coordinate changes across repos when modifying agent protocols.

Stay within these guidelines to keep the controller stable and compatible with the agent network.
