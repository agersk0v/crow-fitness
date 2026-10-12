# Product requirements

## Status and purpose

Crow Fitness is a minimal workout tracker for one person: create a reusable workout template, start a session, record weight and reps, and inspect exercise progress. The runnable infrastructure and connection-check page are implemented; workout features are not implemented yet.

The [README](../README.md) gives the overview. [ARCHITECTURE.md](ARCHITECTURE.md) describes a proposed implementation. This document separates confirmed requirements from proposed MVP behavior; the decisions below remain open until the owner explicitly approves them. Merging a technical proposal does not by itself resolve product choices.

## Confirmed requirements

| ID | Requirement |
| --- | --- |
| R1 | Create reusable workout templates containing exercises. |
| R2 | Start a workout session from a template. |
| R3 | In a session, see which exercises to perform and previous sets for those exercises. |
| R4 | Record weight and reps for performed sets. |
| R5 | Browse recorded sets by exercise to track progress. |
| R6 | Treat all data as belonging to one implicit user, with no authentication, accounts, or multiple-user support. |
| R7 | Build the frontend with Next.js, TypeScript, and Tailwind CSS, the REST API with Go, and persistence with PostgreSQL. |

The technology stack is a constraint; the rest of this document describes user-visible behavior rather than API or database details.

## Proposed pages and navigation

All pages and routes here are proposals for review.

| Page | Route | Purpose |
| --- | --- | --- |
| Workout templates | `/templates` | List, create, edit, delete, and start workouts from templates |
| Workout session | `/sessions/[id]` | View exercises, reference previous performance, and record sets |
| Workout history | `/sessions` | List active/completed sessions and open a workout to review |
| Exercises | `/exercises` | Browse the exercise library |
| Exercise progress | `/exercises/[id]` | View all recorded sets for one exercise, grouped by workout |

Use simple navigation between templates, workout history, and exercises. An empty page explains how to create the first exercise/template or record the first workout. Workout entry should be usable on a phone, with clearly labeled fields and actions.

## Main flows and acceptance criteria

The criteria below define the proposed MVP flows. Criteria involving an open decision are conditional on approval of that decision.

### 1. Create and maintain a template

Create/select exercises from a shared library, name the template, and arrange exercises in the desired order. Under D2, give each exercise a positive planned set count. Templates define exercises and planned rows; they do not prescribe target weights or reps in this proposal.

- A template with a name and at least one exercise can be saved and reopened with the same order and configuration.
- Missing/blank names, missing exercises, and invalid planned set counts show useful validation messages without losing the form input.
- Editing a template changes future sessions only; an existing session keeps the configuration it started with.
- Deleting a template requires confirmation and removes it from the template list. It preserves already started sessions and their recorded history.
- Choosing the same library exercise in different templates uses the same exercise identity for history.

### 2. Start or resume a workout

Choose Start Workout on a template. Create a session with its own copy of the template name, ordered exercises, exercise names, and planned set counts, then open that session.

- The new session shows the exercises in the template's order.
- A later template edit, rename, or deletion does not rewrite the session.
- A failed start does not leave a partially created workout; retrying the same start request does not create duplicates.
- Reopening or refreshing an active session restores its persisted performed sets.
- Under D3, starting another workout while one is active offers to open the existing session and does not create a second active workout.

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
- Editing or deleting a template leaves the completed session unchanged.

### 5. Inspect workout history and exercise progress

History lists sessions by date, copied template name, and active/completed status. Exercise progress groups persisted sets by workout and displays date, weight, reps, and workout status.

- Opening a session shows exactly the sets recorded for that workout.
- The same exercise's recorded sets appear together even if it was used in different templates.
- Under D8, progress includes persisted sets from active and completed workouts, labeled by status; previous-performance references still use completed workouts only.
- Removed sets and unrecorded suggestions are absent from progress.
- If history spans several pages, loading more exposes older records without duplicates or omissions.
- Exercises without recorded sets show an empty state. A readable set history satisfies the MVP; charts and strength estimates are optional later work.

## Decisions requiring review

These defaults match the current architecture proposal. They are recommendations, not approved requirements.

| ID | Decision | Proposed default |
| --- | --- | --- |
| D1 | Initial weight/reps | Prefill from previous completed performance as unrecorded suggestions; unmatched rows stay blank |
| D2 | Planned sets | A positive planned set count per template exercise; allow extra performed sets |
| D3 | Active workouts | Only one active session at a time |
| D4 | Weight units | Kilograms with at most two decimal places, including zero |
| D5 | Save behavior | Explicitly record new sets; autosave valid edits to recorded sets; no offline draft guarantee |
| D6 | Previous performance | Most recent completed session with recorded sets for the exercise, regardless of template |
| D7 | Completion | Permit incomplete/empty sessions; keep completed sessions read-only |
| D8 | Progress scope | Include recorded sets from active and completed sessions, with status labels |

A partial previous workout supplies suggestions only for matching rows. If an exercise occurs more than once in that workout, preserve the source occurrence/order when displaying references; confirm how to map those sets into suggestions before implementing prefilling.

The owner should approve or revise D1-D8 before implementation of affected flows. Exercise renaming/deletion, session cancellation/deletion, and corrections to completed workouts need separate requirements if requested. The proposed MVP supports reopening an existing active session, but does not define a separate pause/cancel state.

## Outside the proposed MVP

- Authentication, accounts, multiple users, and sharing/social features.
- RPE/RIR, rest timers, supersets, exercise instructions, and advanced programming.
- Notifications, wearable integrations, imports, and external services.
- Offline synchronization, live collaboration, and multi-device conflict resolution.
- Required charts, personal-record calculations, and estimated one-rep max.
- Editing/deleting completed sessions or deleting referenced exercises.

Local or private-network use is the proposed deployment scope for this application without authentication; the technical setup belongs in the architecture and eventual README.
