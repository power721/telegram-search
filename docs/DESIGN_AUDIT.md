# Dashboard Design Audit

Date: 2026-06-08

## Problems Found

- The dashboard had a basic sidebar-only shell with no top toolbar, no constrained content frame, and repeated per-page header styling.
- Tables used inconsistent custom CSS, larger row spacing, non-sticky headers, weak empty states, and mixed status presentation.
- Filters were mostly unlabeled control rows, which made keyboard and screen-reader navigation weaker.
- Pagination styling was duplicated across Search, Resources, and Tasks.
- Forms in login, setup, account login, and settings screens repeated light-only card styling and did not share field grouping.
- Dark mode depended on browser defaults for many surfaces because most page styles hard-coded light colors.
- Loading states were sparse, especially for table-heavy operational pages.
- Status indicators mixed Naive tags and custom badges, so task, account, channel, and web-access states did not read as one system.

## Improvements Made

- Added a shared enterprise dashboard design system in `web/src/styles/base.css` with GitHub/Linear/Vercel-style tokens, subtle borders, compact spacing, focus states, dark-mode variables, table primitives, status pills, skeletons, empty states, pagination, and form grouping.
- Rebuilt `AppLayout.vue` around a fixed sidebar, sticky top toolbar, active navigation state, operational chips, and a consistent max-width content frame.
- Migrated Home, Search, Channels, Resources, Accounts, Tasks, Settings, login, and setup pages to the shared visual language.
- Added Home global search entry and Search route query support for a better search workflow.
- Added labels to Search, Resource, and Channel filter bars for better keyboard and accessibility semantics.
- Standardized table behavior with compact rows, sticky headers, shared table styling, loading skeletons, and richer empty states.
- Improved status indicators for accounts, tasks, channel sync/listen state, web access, and setup channel status.
- Consolidated repeated form/card styling across login, setup, settings, account login modal, and listen-rule forms.
- Added focused tests for the dashboard shell and compact resource table contract.

## Remaining Issues

- Screenshots are captured from a static visual harness because the live app requires backend auth/setup state; full visual regression coverage would be stronger with authenticated Playwright fixtures.
- Table sorting is still implemented locally in Channels only; Resources and Tasks keep their existing backend/order behavior.
- Settings still exposes only current storage/API-key controls available in the app; deeper settings sections would need backend/API expansion.
- The design system now centralizes primitives, but a future pass could extract Vue components for table pagination, empty states, and page headers to reduce markup duplication further.
