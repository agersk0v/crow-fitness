# Product requirements

## Status and purpose

Crow Fitness is a minimal workout tracker for one person: create a reusable workout template, start a session, record weight and reps, and inspect exercise progress. Connection checks, exercise management, and template management are implemented. One persisted active session can be started, continued, cancelled, or completed. Active-session set entry, workout history with a Sets tab, and previous-performance references are implemented. Exercise-specific history/progress remain planned.

The [README](../README.md) gives the overview. [ARCHITECTURE.md](ARCHITECTURE.md) separates the implemented design from future proposals. This document separates confirmed requirements from proposed MVP behavior; the decisions below remain open until the owner explicitly approves them. Merging a technical proposal does not by itself resolve product choices.

## Confirmed requirements

| ID | Requirement |
| --- | --- |
| R1 | Create reusable workout templates containing exercises. |
| R2 | Start a workout session from a template. |
| R3 | In a session, see which exercises to perform and previous sets for those exercises. |
| R4 | Record weight and reps for performed sets. |
| R5 | Browse recorded sets by exercise to track progress. |
| R6 | Exercises are personal and belong to an internal owner. Go selects one fixed local development owner for #11. Login and future multiple-user authentication remain separate work. |
| R7 | Build the frontend with Next.js, TypeScript, and Tailwind CSS, the REST API with Go, and persistence with PostgreSQL. |

The technology stack is a constraint; the rest of this document describes user-visible behavior rather than API or database details.

## Approved exercise ownership (#11)

The owner approved personal exercises with one internal local owner on 2026-10-13. Go selects that owner for every create/list operation; browser-supplied identity is not accepted. Exercises are not shared across future users. The internal owner does not provide authentication or access protection.

## Approved template targets (#23)

A template has a nonblank name and at least one ordered saved exercise. Working-set counts are required. Rep ranges and target RIR are optional and belong to the exercise occurrence within the template, so different templates or repeated occurrences can use different targets. One target applies across its working sets; no individual-set targets or warm-up sets. Target RIR is distinct from performed RIR. Notes, rest times, and planned weights are deferred. Template management is defined below. Zero RIR is valid; decimals are accepted without a product-specific upper limit or rounding rule.

## Approved exercise management (#27)

Creating or editing an exercise requires one or more tags from the confirmed predefined list: chest, back, shoulders, biceps, triceps, forearms, core, glutes, quadriceps, hamstrings, and calves. Custom tags and primary/secondary distinctions are deferred. Case-insensitive name search combines with a single muscle-group filter; clearing them restores the list.

Renaming preserves exercise identity and updates template displays. Edit offers Save/Cancel, inline validation, saving feedback, and retained drafts for retry. Delete requires confirmation naming the exercise and is blocked when any template references it; show links to those templates, counting repeated occurrences once. Concurrent requests must not break references or leave partial templates. Existing records receive empty tags during migration and remain available; they must be tagged the next time they are edited. Future recorded workouts must retain their captured names/history; active sessions preserve copied exercise names; set entry and retention follow #33 below.

## Approved template management (#29)

Active templates can change their names, exercise occurrences/order, working-set counts, and optional rep/RIR targets. Reorder occurrences by dragging the card background, holding briefly before touch dragging, or using the keyboard. Inputs, labels, dropdowns, and buttons do not start a drag; ordinary touch swipes can still scroll. Save applies the complete draft atomically; Cancel discards it, and failures retain it for retry. Removing an occurrence leaves its exercise library record intact. Existing validation rules and ownership apply.

Duplication creates an independent template with a supplied nonblank name and the same ordered configuration and targets. Later changes or archiving of either template do not affect the other. Names are not required to be globally unique.

Archive requires confirmation naming the template and preserves its identity, configuration, and exercise references. Active is the default list; Archived provides read-only inspection and restoration. Restore preserves the same identity and configuration. Restore before editing, duplicating, or starting a workout. Archived templates still block deletion of referenced exercises, with archived status shown in usage information. Permanent template deletion is out of scope.

Future recorded workouts must preserve their snapshots/history through template edits and archiving. Active sessions follow #31 below; completed sets follow #33 below.

## Approved active-session lifecycle (#31, superseded completion in #33)

Start a workout from an active template and allow only one active session for the local owner. Starting while one exists offers continuation and never replaces it. Save a snapshot of the template name, ordered exercises and their names, working-set counts, and optional rep/RIR targets. Repeated occurrences and blank targets are preserved. Later template edits, exercise renames/removal from the library, or template archiving leave this session unchanged.

Home shows Continue session when one exists. Refresh, reopening the app, and database/stack restarts retain it. The session page displays its copied configuration and offers Continue later, Cancel session, and Complete session. Cancel requires confirmation; dismissing it preserves the session. Confirmed cancellation discards the session/sets; completion now retains them under #33 and returns home, allowing another workout to start. Failed requests show retryable errors; ending an old session cannot clear a newer one.

Previous performance follows #35 below. Changing exercises/targets during a session, timers, and a detailed workout-history interface remain outside this increment. Future proposals below do not override the approved lifecycle.

## Approved session sets and completion (#33)

The owner confirmed the recommended defaults on 2026-10-13: kilograms with up to two decimal places (zero is valid), positive whole-number reps, explicit Add/Save actions for creation and editing, completion with fewer or no sets, and workout completion dates displayed in the browser's local timezone.

Sets belong to a specific captured exercise occurrence, including repeated exercises. Saving keeps them in the active session for refresh/continuation; they are not yet on the Sets page. Active sets can be edited or deleted. Completion preserves the snapshot and saved sets, records completion once, and clears the active slot. It never converts targets or unsaved inputs into performed results. Cancel confirmation explains that cancellation discards the session and all its sets; Keep/Escape preserves them.

Completion waits for successful pending changes. Open drafts and failed changes remain available and block completion until saved, explicitly discarded, or resolved. Retrying uncertain writes must not duplicate sets or recreate deleted sets. Completed sets cannot be edited through active-session endpoints, and stale cancellation cannot delete a completed workout or newer session.

Home links to a simple Sets page: only completed-workout sets, newest workouts first, showing copied exercise name, weight, reps, and workout completion date. Empty completed workouts produce no set rows. No filters, charts, timers, actual RIR, reference suggestions, session exercise editing, or detailed history interface in this issue. Later template/exercise changes or archiving preserve completed snapshots/results.

## Approved previous performance (#35)

The owner confirmed the selection rule: use the latest completed workout containing at least one nondeleted saved set for the same exercise identity, across any template. Active/cancelled sessions and completed workouts with no eligible sets for that exercise are excluded. Completion time and session identity break recency ties. Renaming an exercise does not disconnect its history.

Each active exercise occurrence has a read-only Previous performance section with the source completion date and exact weight/reps. Repeated source occurrences are shown separately in their original occurrence/set order; repeated current occurrences may share the same reference. An exercise without eligible history shows No previous performance yet. Previous results are visually separate from current sets and template targets.

References load independently of current-session data. Failed loads show retry while set controls and drafts remain usable. Retry does not reload/reset current input; refresh and Continue session load references alongside saved current sets. Reading or completing never copies previous results into current records. Prefilling, copying, and progression recommendations remain separate work.

## Approved workout history (#37)

The owner confirmed Sessions as the first tab. Home's Workout history link opens `/history`; Sessions and Sets tabs store selection in the URL for refresh and back navigation. Sessions lists completed workouts as template-style cards using captured names and local completion dates, newest first. Repeated workouts remain separate records. Sets retains the plain completed-set list. Active and cancelled sessions appear in neither tab.

A session card opens a read-only saved snapshot with ordered exercise occurrences, captured names, template targets, and actually recorded sets in separate sections. Empty completed workouts still appear in Sessions and explain that no sets were recorded; they contribute no Sets rows. Source edits, archiving, renames, or removal of an otherwise unused exercise do not change history.

Tabs support keyboard navigation, selected/focus states, empty states, and retryable load errors. Missing/foreign detail IDs show not found. `/sets` redirects to the Sets tab so old links still work. Completed views offer no workout/set/template mutations. Exercise-specific history, filters, charts, analytics, correction, and creating templates from workouts remain separate work.

## Pages and navigation

Home links to exercises and templates and offers continuation of an active session. Workout history is implemented; exercise progress remains proposed.

| Page | Route | Purpose |
| --- | --- | --- |
| Workout templates | `/templates`, `/templates/[id]` | Create, edit, duplicate, archive, inspect, restore, start (implemented) |
| Workout session | `/sessions/[id]` | View the active snapshot, add/edit/delete sets, continue later, cancel, or complete (implemented) |
| Completed sets | `/history?tab=sets`; `/sets` redirects | Plain completed-set list (implemented) |
| Workout history | `/history`, `/history/[id]` | Completed session cards and read-only snapshots (implemented) |
| Exercises | `/exercises` | Create, search/filter, tag, rename, and safely delete personal exercises (implemented) |
| Exercise progress | `/exercises/[id]` | View all recorded sets for one exercise, grouped by workout |

Use simple navigation between templates, workout history, and exercises. An empty page explains how to create the first exercise/template or record the first workout. Workout entry should be usable on a phone, with clearly labeled fields and actions.

## Main flows and acceptance criteria

The criteria below define the proposed MVP flows. Criteria involving an open decision are conditional on approval of that decision.

### 1. Create and maintain a template

Template creation and management follow the approved sections above. Templates do not prescribe target weights. Active-session behavior follows #31; history/logging criteria remain proposals.

- A template with a name and at least one exercise can be saved and reopened with the same order and configuration.
- Missing/blank names, missing exercises, and invalid planned set counts show useful validation messages without losing the form input.
- Editing a template changes future sessions only; an existing session keeps the configuration it started with.
- Archiving a template requires confirmation and hides it from Active without removing references. Future sessions/history must be preserved.
- Choosing the same library exercise in different templates uses the same exercise identity for history.

### 2. Start or resume a workout

The approved #31 lifecycle above is implemented. Start Workout opens a saved snapshot; Continue session from home reopens it. Saved current sets and previous-performance references are restored. Retries while a session exists offer that same session instead of creating a duplicate.

### 3. Save sets in the active session

The approved #33 behavior above is implemented. Add/Save records weight/reps in the active session; Edit/Save replaces values and Delete removes the set. Read-only references follow #35; suggested/prefilled values remain future work.

### 4. Complete a workout

The approved #33 behavior above is implemented. Complete retains the workout and its saved sets; Cancel discards them. Read-only completed-workout inspection is implemented in #37; corrections remain deferred.

### 5. Inspect workout history and exercise progress

Workout history follows the approved #37 behavior above. Completed sessions and sets are read-only. Exercise-specific history/progress remains separate work; charts and strength estimates are not required.

## Decisions requiring review

The following logging/history defaults remain recommendations, not approved requirements. One active session is approved in #31. #33 approves retaining completed results, discarding cancelled workouts, and the defaults stated above. The template working-set count is already approved; D2 now concerns extra performed sets.

| ID | Decision | Proposed default |
| --- | --- | --- |
| D1 | Initial weight/reps | Prefill from previous completed performance as unrecorded suggestions; unmatched rows stay blank |
| D2 | Planned sets | Positive working-set counts per template exercise are approved in #23; extra performed sets in sessions remain proposed |
| D8 | Future exercise progress scope | Include active/completed sets with labels remains proposed; the current Sets page includes completed sets only |

A partial previous workout supplies suggestions only for matching rows. If an exercise occurs more than once in that workout, preserve the source occurrence/order when displaying references; confirm how to map those sets into suggestions before implementing prefilling.

Resolve the remaining decisions before implementing affected session flows. Active-session cancellation is approved in #31. Corrections to future completed workouts need separate requirements. Continue later retains the active session without introducing a separate pause state.

## Outside the proposed MVP

- Authentication, accounts, multiple users, and sharing/social features.
- Performed RPE/RIR, rest timers, supersets, exercise instructions, and advanced programming. Optional template target RIR is already approved and implemented.
- Notifications, wearable integrations, imports, and external services.
- Offline synchronization, live collaboration, and multi-device conflict resolution.
- Required charts, personal-record calculations, and estimated one-rep max.
- Editing/deleting completed sessions or deleting referenced exercises.

Local or private-network use is the proposed deployment scope for this application without authentication; the technical setup belongs in the architecture and eventual README.
