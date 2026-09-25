# Web Frontend Development Guide

`index.html` and `src/main.ts` are the only build entry points. `pnpm run build` builds both frontends in one pass; output remains in `internal/webui/dist` and is delivered with the Go binary.

## Code boundaries

- `src/frontends/classic/`: retains the original frontend in full. The existing `@/` alias points here to avoid unrelated path rewrites. Do not change original pages, interactions, or functionality other than interface switching settings.
- `src/frontends/modern/`: an independent modern frontend with its own bootstrap, routing, layouts, components, business features, API types, styles, and copy. `@modern/` points here.
- `src/shared/`: HTTP client, base protocols, the British English locale, and browser interface preferences. `@shared/` points here. Page response DTOs, business query orchestration, and components do not belong here.

Both frontends may depend on shared, but must not depend on each other; shared must not depend on a frontend or bootstrap entry point. The existing ESLint command checks path boundaries for static imports, exports, dynamic imports, and resource globs.

The two frontends have separate style entry points and only the chosen interface starts. Classic Tailwind source scanning is limited to classic; the entry point and shared do not import either frontend's styles.

## Interface switching

The browser interface preference uses `gpt-load.frontend.v2`: choosing classic stores `classic`; choosing modern clears the value; modern is the default. The old `gpt-load.frontend` key is cleared at startup and no longer participates in interface selection. The sign-in page and access-key sessions always use modern; only administrators can choose their interface on the settings page. Saving the choice reloads `/settings`; each load creates only one application and router instance.

Only when a classic preference and sign-in credential both exist does the bootstrap selector request `/api/auth/session`, waiting at most five seconds including response-body reading. Confirmed administrators load classic; confirmed access-key identities or a 401 clear the interface preference. Storage, network, timeout, temporary HTTP, or response-format failures return to modern and retain the original choice, where the modern authentication page provides recovery. The theme retains its established browser storage convention.

Classic global settings include interface selection and preview thumbnails. Unsaved changes or active settings operations must first complete their established save-or-discard flow. If the preference cannot be saved, an error remains visible and the current interface is retained.

## Current scope

Modern provides the Coral Anchor brand frame: a sidebar collapse control centred in the separator, a mobile navigation drawer, and a broad workspace. The top bar is shared across pages: current page title on the left, then data update time and refresh, followed by an import-key entry point, theme, and sign out. Global settings retain interface switching. Navigation displays Workspace, Observability, and System directly instead of using secondary tabs for observability pages.

`app/navigation.ts` describes navigation entries centrally. `internal/webui/page_routes.json` retains classic's original nine page addresses; modern-only `/monitor/usage`, `/monitor/logs`, `/monitor/health`, and `/monitor/inspector` live in `internal/webui/modern_page_routes.json`. The server and modern frontend read both manifests together; classic reads only the original manifest, preventing added pages from triggering its route-completeness check. Every page still serves the same HTML entry point; `/monitor` redirects to usage in modern while preserving classic behaviour.

Modern provides an overview, group lists and details, access keys, model management, request logs, usage statistics, runtime health, and global settings. The overview organises client connection configuration, request trends, subscription-account quota, and health tasks; client selection and route inspection are independent. `/monitor/inspector` redirects to the home route-inspection area while retaining parameters; `/import` redirects to the group import entry point, where the user creates a group or adds credentials to an existing one. Failed route loading offers refresh and retry.

The group workspace uses a two-line list: group and channel, credential total and status progress, model count and pricing multiplier, 24-hour requests and usage, enablement, and inline weighting. It supports local search, channel and runtime-view filters, sorting, copying addresses, and expanding the full model list in place. A complete light snapshot is fetched once; the default page size is 20 with 50 and 100 available. The header and bottom pagination remain fixed while only the content scrolls. Basic editing happens in group details; models and advanced configuration use independent side panels. API keys and subscription accounts each provide cards, filtering, pagination, details, and batch operations; subscription-account synchronisation states remain independent.

Model cards show upstream sources, linked groups, and pricing rules. Logs support cursor pagination, details, and quick field filters; usage shows request success and failure, input and output tokens, cache and cost trends, and source rankings. The health page and home tasks share `/api/health`, which returns complete exception details; the browser handles filtering and pagination without limiting backend report counts. The backend principal-permission contract continues to constrain business data.

Theme is a local browser preference and can be adjusted on the sign-in page; global settings retain administrator-only access. Sidebar collapse uses `gpt-load.modern.sidebar-collapsed`; interface preferences do not read or write server configuration. Both settings pages use preview thumbnails from the shared interface directory. The modern brand component reacts to the actual theme, sidebar collapse, and expansion, and observes reduced-motion preferences.

The sidebar footer links only to Demerzel documentation and the GitHub repository; upstream sponsorship and community links are not project entry points. GitHub uses the official Octicons mark. The separator shows only the running version returned by `/health`; the application does not call the old classic administrator update-check endpoint or access release sources automatically. Downloading and upgrades use controlled installation procedures.

Modern pages may independently design display DTOs, aggregate queries, and management interfaces. `modern/api/` is their separate API directory. Group display uses `/api/modern/groups`; current-page usage is read in batches through `/api/modern/groups/usage`, with at most 100 groups per request, reusing the backend's precise time window and final request attribution. Boundary-detail queries use a combined group and completion-time index, avoiding per-group scans or full ranking calculation. Credential display uses `/api/modern/groups/:group_id/credentials` and its detail endpoint to enrich activity data for the current page. Enablement, disablement, and editing continue to use existing management interfaces, preserving old interface contracts, core flows, and data-plane behaviour.

## Sign-in and permissions

Modern sign-in is independently implemented in `modern/features/auth/`. It calls the existing `GET /api/auth/session` and retains the `gpt-load.auth-key` storage key. Remember sign-in is unchecked by default and uses sessionStorage for the current browser-tab session, including after refresh; when selected, it uses localStorage. The selected store is written only after successful authentication with an `admin` or `access_key` identity, and old credentials are cleared from the other store. Restoration reads the current tab's sessionStorage first and then localStorage; sign-out or authentication failure clears both stores.

Empty values, whitespace, invalid keys, lockout, network errors, and invalid responses retain their distinct feedback; failed sign-in does not alter an existing credential. Manual retry becomes available when the lockout countdown finishes, and `/login?help=auth` opens key-source help directly.

A refreshed key is revalidated before mounting the workspace and business components; concurrent restoration sends one validation request. Network and response failures retain the key and show a retry entry point. Authentication failure clears the session, cancels and clears the modern query cache, and returns to sign-in; a 403 permission denial does not sign out by mistake. Successful sign-in returns only to known protected in-site paths while retaining query and hash; administrator-only paths return access-key users to the overview.

`app/navigation.ts` uses `adminOnly` to constrain routes, the sidebar, and quick navigation. Administrators can access every entry point; access keys can access only overview, models, usage, and logs, while the backend still validates identity scope. The top-right sign-out action retains the session if navigation fails or is blocked; after successful exit it clears authentication and cache.

The HTTP client captures the session revision per request; a late 401 cannot clear a new session after identity changes or request cancellation. Session restoration and sign-in responses also check revision and cancellation state, preventing an old response from signing in again after sign-out. When the selected store is unavailable, an in-memory session remains; an unremembered sign-in does not fall back to localStorage. Business code and revision checks obtain credentials from the session client rather than reading browser storage directly. Authentication logic and components do not depend on classic; the shared HTTP layer provides the base request contract while the server remains responsible for authentication and interface permissions.

## Validation

The frontend uses the existing lint, format, type-check, and build commands; run repository `make check` at the final integration stage. Do not add frontend tests, browser acceptance, or local race checks.
