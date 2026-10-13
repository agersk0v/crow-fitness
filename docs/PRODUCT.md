# Product requirements

## Status and purpose

Crow Fitness is a minimal workout tracker for one person: create a reusable workout template, start a session, record weight and reps, and inspect exercise progress. Connection checks, exercise management, and template management are implemented. One persisted active session can be started, continued, cancelled, or completed. Set logging and workout history remain planned.

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

A template has a nonblank name and at least one ordered saved exercise. Working-set counts are required. Rep ranges and target RIR are optional and belong to the exercise occurrence within the template, so different templates or repeated occurrences can use different targets. One target applies across its working sets; no individual-set targets or warm-up sets. Target RIR is distinct from performed RIR. Notes, rest times, planned weights, and logging are deferred. Template management is defined below. Zero RIR is valid; decimals are accepted without a product-specific upper limit or rounding rule.

## Approved exercise management (#27)

Creating or editing an exercise requires one or more tags from the confirmed predefined list: chest, back, shoulders, biceps, triceps, forearms, core, glutes, quadriceps, hamstrings, and calves. Custom tags and primary/secondary distinctions are deferred. Case-insensitive name search combines with a single muscle-group filter; clearing them restores the list.

Renaming preserves exercise identity and updates template displays. Edit offers Save/Cancel, inline validation, saving feedback, and retained drafts for retry. Delete requires confirmation naming the exercise and is blocked when any template references it; show links to those templates, counting repeated occurrences once. Concurrent requests must not break references or leave partial templates. Existing records receive empty tags during migration and remain available; they must be tagged the next time they are edited. Future recorded workouts must retain their captured names/history; active sessions preserve copied exercise names; set logging remains unimplemented.

## Approved template management (#29)

Active templates can change their names, exercise occurrences/order, working-set counts, and optional rep/RIR targets. Reorder occurrences by dragging the card background, holding briefly before touch dragging, or using the keyboard. Inputs, labels, dropdowns, and buttons do not start a drag; ordinary touch swipes can still scroll. Save applies the complete draft atomically; Cancel discards it, and failures retain it for retry. Removing an occurrence leaves its exercise library record intact. Existing validation rules and ownership apply.

Duplication creates an independent template with a supplied nonblank name and the same ordered configuration and targets. Later changes or archiving of either template do not affect the other. Names are not required to be globally unique.

Archive requires confirmation naming the template and preserves its identity, configuration, and exercise references. Active is the default list; Archived provides read-only inspection and restoration. Restore preserves the same identity and configuration. Restore before editing, duplicating, or starting a workout. Archived templates still block deletion of referenced exercises, with archived status shown in usage information. Permanent template deletion is out of scope.

Future recorded workouts must preserve their snapshots/history through template edits and archiving. Active sessions follow #31 below; completed workout history remains deferred.

## Approved active-session lifecycle (#31)

Start a workout from an active template and allow only one active session for the local owner. Starting while one exists offers continuation and never replaces it. Save a snapshot of the template name, ordered exercises and their names, working-set counts, and optional rep/RIR targets. Repeated occurrences and blank targets are preserved. Later template edits, exercise renames/removal from the library, or template archiving leave this session unchanged.

Home shows Continue session when one exists. Refresh, reopening the app, and database/stack restarts retain it. The session page displays its copied configuration and offers Continue later, Cancel session, and Complete session. Cancel requires confirmation; dismissing it preserves the session. Confirmed cancellation and completion clear the active session and return home, allowing another workout to start. No completed record, set result, or history is saved. Failed requests show retryable errors; ending an old session cannot clear a newer one.

Set logging, previous performance, changing exercises/targets during a session, timers, and completed-session history are outside this increment. Future proposals below do not override this approved lifecycle.

## Pages and navigation

Home links to exercises and templates and offers continuation of an active session. History and progress routes below remain proposals.

| Page | Route | Purpose |
| --- | --- | --- |
| Workout templates | `/templates`, `/templates/[id]` | Create, edit, duplicate, archive, inspect, restore, start (implemented) |
| Workout session | `/sessions/[id]` | View the active snapshot, continue later, cancel, or complete (implemented); logging remains proposed |
| Workout history | `/sessions` | Browse completed workouts (proposed) |
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

The approved #31 lifecycle above is implemented. Start Workout opens a saved snapshot; Continue session from home reopens it. There are no performed sets to restore yet. Retries while a session exists offer that same session instead of creating a duplicate.

### 3. View reference performance and record sets

Each exercise shows its recorded sets alongside a separate reference to previous performance. Under D1 and D2, planned rows may start with suggested values. A suggestion is never proof that the set was performed.

- Each performed set records weight and reps and belongs to a specific exercise occurrence in the workout.
- Under D6, previous performance comes from the most recent completed workout containing recorded sets for that exercise, across any template. An exercise with no history shows a clear empty state.
- Suggested values remain visibly unrecorded until the user selects Record/Done. They do not appear in history or totals, and completing a workout does not silently record them.
- A recorded set can be edited or removed while the session is active. Extra sets can be recorded beyond the planned count. Removing a set removes it from history without altering other recorded weights/reps.
- Reps must be a positive whole number. Under D4, weight must be nonnegative kilograms with at most two decimal places; zero is allowed for bodyweight exercises.
- Invalid input stays visible with a field-level error and is not saved.
- Under D5, new sets use explicit recording; valid edits to recorded sets autosave. Show saving, saved, and failed states. Failed saves retain the input and allow retry.
- Retrying a save after a lost response does not create a duplicate performed set. Refresh restores successfully persisted values; unsaved drafts may be lost.

### 4. Complete and review a workout

Choose Finish Workout once done. Under D7, fewer than the planned sets, including none, are allowed. Preserve the session and whatever sets were actually recorded.

- Completion waits for pending saves. A failed save prevents finishing until it is retried or the user explicitly discards the unsaved change.
- Completion records a finish time and marks the session completed; repeating completion does not create another workout or change its finish time.
- Unrecorded suggested/planned rows do not become performed sets.
- A completed session opens as a read-only record of its copied exercise configuration and recorded sets.
- Editing or archiving a template leaves the completed session unchanged.

### 5. Inspect workout history and exercise progress

History lists sessions by date, copied template name, and active/completed status. Exercise progress groups persisted sets by workout and displays date, weight, reps, and workout status.

- Opening a session shows exactly the sets recorded for that workout.
- The same exercise's recorded sets appear together even if it was used in different templates.
- Under D8, progress includes persisted sets from active and completed workouts, labeled by status; previous-performance references still use completed workouts only.
- Removed sets and unrecorded suggestions are absent from progress.
- If history spans several pages, loading more exposes older records without duplicates or omissions.
- Exercises without recorded sets show an empty state. A readable set history satisfies the MVP; charts and strength estimates are optional later work.

## Decisions requiring review

The following logging/history defaults remain recommendations, not approved requirements. One active session and clearing it on completion/cancellation are approved in #31. The template working-set count is already approved; D2 now concerns extra performed sets.

| ID | Decision | Proposed default |
| --- | --- | --- |
| D1 | Initial weight/reps | Prefill from previous completed performance as unrecorded suggestions; unmatched rows stay blank |
| D2 | Planned sets | Positive working-set counts per template exercise are approved in #23; extra performed sets in sessions remain proposed |
| D4 | Weight units | Kilograms with at most two decimal places, including zero |
| D5 | Save behavior | Explicitly record new sets; autosave valid edits to recorded sets; no offline draft guarantee |
| D6 | Previous performance | Most recent completed session with recorded sets for the exercise, regardless of template |
| D7 | Completion | Permit incomplete/empty sessions; keep completed sessions read-only |
| D8 | Progress scope | Include recorded sets from active and completed sessions, with status labels |

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
