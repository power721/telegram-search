# Mobile Responsiveness Design

**Date:** 2026-06-13
**Status:** Approved
**Approach:** File-by-file, following existing ResourceTable.vue card-layout pattern

## Context

Desktop-first Vue 3 app with ~15 existing media queries. Proactive polish pass to ensure full usability on 320px–768px screens. No known breakage, but several tables force horizontal scrolling, drawers overflow small viewports, and touch targets are undersized.

## Breakpoints

- **760px** — primary mobile breakpoint (consistent with existing code)
- **480px** — extra-small adjustments (nav density)
- **900px** — existing tablet breakpoint (no changes)

## Changes by File

### 1. layouts/AppLayout.vue — Nav sizing
- At `max-width: 760px`: reduce `.nav-item` `min-width` from 92px → 64px
- At `max-width: 480px`: hide eyebrow text (`.eyebrow`), show icon + label only

### 2. views/ChannelsView.vue — Card layout for channels table
- Table has `min-width: 980px` with no mobile fallback
- At `max-width: 760px`: hide table, show `.mobile-cards` container
- Each card: avatar + name + username header, badge row (sync status, web access), stats + last sync, action buttons in footer row

### 3. views/AccountsView.vue — Card layout for accounts table
- Table has `min-width: 760px` with no mobile fallback
- At `max-width: 760px`: hide table, show `.mobile-cards` container
- Each card: avatar + phone + name header, status badge, last online, action buttons in footer row

### 4. components/tasks/TaskTable.vue — Card layout for tasks table
- Table has `min-width: 1120px` with no mobile fallback
- At `max-width: 760px`: hide table, show `.mobile-cards` container
- Each card: ID + type header with status badge, progress bar, timestamps + retry count, action buttons in footer row
- Bulk selection checkbox preserved as first element in each card

### 5. views/LogsView.vue — Simplified log rows on mobile
- 6-column table with `min-width: 126px` time column and `max-width: 520px/620px` message columns
- At `max-width: 760px`: hide file, caller, and fields columns; show only time + level badge + message
- Allow message text to wrap naturally (remove `max-width`)

### 6. views/ApiHelpView.vue — Remove fixed column widths on mobile
- Last column has `min-width: 320px` — wider than 320px viewport
- At `max-width: 760px`: remove `min-width` on last column, allow natural wrapping

### 7. views/SettingsView.vue — Responsive tables and touch targets
- Settings tables have `min-width: 680px`
- At `max-width: 760px`: reduce to `min-width: auto`, allow natural table sizing
- Increase checkbox inputs from 16×16 to 20×20px on mobile

### 8. components/channels/ChannelControlDrawer.vue — Responsive drawer width
- Fixed `width="420"` overflows 320px viewport
- Use computed width: `Math.min(420, window.innerWidth * 0.9)` — 90vw on mobile, 420px on desktop

### 9. components/tasks/TaskDetailDrawer.vue — Responsive drawer width
- Fixed `width="520"` overflows 375px viewport
- Use computed width: `Math.min(520, window.innerWidth * 0.9)` — 90vw on mobile, 520px on desktop

### 10. components/resources/ResourceTable.vue — Larger checkboxes on mobile
- Checkbox inputs are 16×16px
- At `max-width: 760px`: increase to 20×20px

### 11. styles/base.css — Global mobile touch target rules
- At `max-width: 760px`: `input[type="checkbox"]` gets `width: 20px; height: 20px`
- Add `min-height: 44px` to checkbox labels for Apple HIG touch targets

## Card Layout Pattern (from ResourceTable.vue)

```css
@media (max-width: 760px) {
  .desktop-table { display: none; }
  .mobile-cards { display: flex; flex-direction: column; gap: 8px; }
  .mobile-card {
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 12px;
  }
  .mobile-card-header { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
  .mobile-card-meta { font-size: 11px; color: var(--muted); }
  .mobile-card-actions {
    display: flex; gap: 6px; margin-top: 8px;
    border-top: 1px solid var(--border); padding-top: 8px;
  }
}
```

## Out of Scope

- Setup/auth views (LoginView, Setup* views — simple forms, already work)
- HomeView, SearchView, ResourcesView (already responsive)
- No new components or shared abstractions
- No viewport meta tag changes
- No routing changes
- No dedicated mobile navigation pattern (hamburger menu, bottom nav)

## Supported Browsers

- Desktop: Chrome, Edge, Firefox, Safari
- Mobile: Chrome (Android), Safari (iOS)

## Testing

- Verify via browser device emulation at 320px, 375px, 768px, 1024px
- Confirm no regressions at desktop widths (1280px, 1920px)
- Manual touch interaction testing on real device or emulation
