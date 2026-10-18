# Product behavior

This is the authoritative description of the current app on `main`. [README](../README.md) owns setup and check commands; [ARCHITECTURE](ARCHITECTURE.md) owns technical contracts. Future work below does not override implemented rules.

## Scope and navigation

Crow Fitness is a minimal personal workout tracker: maintain exercises, build reusable templates, record one active workout, and inspect completed results. Go selects one internal local owner. There is no authentication, account management, sharing, or multiple-user runtime support; the owner ID is not access protection.

Every page shares Home, Exercises, Templates, and Workout history navigation plus a light/dark toggle. Detail pages highlight their parent section; active sessions highlight Templates. Dark is the default and the browser remembers the chosen theme. Controls support keyboard focus and phone layouts; motion respects reduced-motion preferences.

| Route | Purpose |
| --- | --- |
| `/` | Connection checks, active-session continuation, optional testing reset |
| `/exercises` | Exercise library and management |
| `/templates` | Active/archived template overviews and creation |
| `/templates/[id]` | Edit an active template or inspect/restore an archived one |
| `/sessions/[id]` | Active workout and set entry |
| `/history` | Sets first/default, Sessions second |
| `/history/[id]` | Read-only completed snapshot |
| `/sets` | Compatibility redirect to `/history?tab=sets` |

`/exercises/[id]/history` and `/api/exercises/{id}/history` are removed and return 404. Exercise browsing uses `/history?tab=sets&exercise=<id>`.

## Exercise library

Exercises require a trimmed nonblank name and at least one predefined muscle group: chest, back, shoulders, biceps, triceps, forearms, core, glutes, quadriceps, hamstrings, calves. Multiple groups are allowed; custom groups and primary/secondary distinctions are not implemented. Duplicate names are allowed. Legacy exercises with no tags remain readable and must receive a tag when edited.

Optional descriptions are plain multiline text, trimmed at the outside, with no reserved card area when absent. They belong to library metadata, are not searched, and are not copied into workout snapshots. Exercise and template lists use spaced individual cards.

Case-insensitive name substring search combines with one muscle-group filter. Empty libraries and no matches have distinct states. Clicking a card/name opens Sets history filtered by stable exercise identity, clearing unrelated criteria; Edit/Delete remain independent controls with keyboard/new-tab behavior preserved.

Edit offers Save/Cancel, inline errors, and retained input after failure. Renaming preserves identity and updates template displays, while existing workout snapshots retain their names. Delete requires confirmation and is blocked by references from **any active or archived template**; show those template links and archived status, counting repeated occurrences once. Cancellation/Escape makes no change. Deleting an otherwise unused exercise does not erase recorded snapshots/history.

## Templates

A template requires a trimmed nonblank name and at least one ordered exercise occurrence. Each occurrence has a positive whole-number working-set count, an optional paired positive rep range (equal bounds mean a fixed target), and optional nonnegative decimal target RIR, including zero. One target applies to all working sets of that occurrence. No warm-up sets, individual-set targets, or planned weights are implemented. Each ordered slot has a default exercise and optional alternatives from the library, each with independent targets. The same identity cannot occur twice within a slot, but can appear in different slots with different targets. Names need not be unique.

Descriptions follow the exercise description rules. The overview shows every slot, its default/alternatives, and each choice’s working-set count, omitting unset optional targets. Active cards have exactly **Start workout**, larger and primary at the top right, and **Edit workout** below the list. There are no template accordions or inline editors.

The dedicated editor has Save template at the top, a small red Unsaved changes label beside its title, plus Copy workout and Delete. Exercise occurrence cards resemble the library with name/tags/description and compact targets. Edit opens a target popup to choose the default, add/remove alternatives, and change each choice’s targets; Apply changes updates only the draft, and Cancel/Escape discards popup edits. Delete on an occurrence removes it from the draft, never from the library. Save persists the complete configuration atomically; errors retain the draft.

Add exercise below is at the lower right of each occurrence and inserts after that specific occurrence. An empty template offers Add first exercise. Drag a card's noninteractive background to reorder it, hold briefly on touch, or focus it and use Space/arrows/Space; Escape cancels. Inputs, labels, dropdowns, and buttons keep their normal behavior and touch scrolling remains available. List/editor buttons and links use pointer cursors.

There is no page-level Cancel or navigation confirmation: leaving, refreshing, or Back discards unsaved edits. Copy workout uses the **last saved** slots, defaults, alternatives, targets, and description, explains unsaved changes, requests a new nonblank name, and creates an independent template. Success preserves the original draft and offers a link to the copy; following it leaves the original editor.

Template **Delete means archive**, with confirmation explaining restoration and preserved active/completed history, including unsaved draft loss. Active is the default list; Archived cards show their exercises and View workout. Archived detail is read-only with Restore; restore before editing, copying, or starting. Archiving keeps identity, configuration, and exercise references. Permanent deletion is not implemented. Template edits and archiving affect future starts, never existing workout snapshots.

## Active workout lifecycle

Start workout copies the template name, ordered exercise identities/names, working-set counts, and optional rep/RIR targets into a saved session. Only one session may be active; another start offers Continue session instead of replacing it. Template edits, exercise renames/removal, and archiving do not rewrite the snapshot. Each session also captures every allowed choice and its targets. For slots with alternatives, the entire title row toggles an inline list, with a small Variations hint beside the exercise name. Choices show only their name and Default/Selected markers, separated by spacing and subtle backgrounds rather than borders. Choosing another exercise collapses the list after a confirmed switch; cancelling draft loss keeps the list and inputs. Clicking the title row again or pressing Escape closes it. Single-choice slots keep a plain title. Only captured choices are available, and targets cannot be edited inside a session.

Switching is allowed only with no surviving saved sets in that slot. Delete all saved sets first; a switch never deletes or reassigns sets automatically. Pending/uncertain writes, open edits, failed deletion/order, or unresolved recovery block switching for that slot. Unsaved entry values require Discard inputs and switch; Keep inputs/Escape preserves selection and inputs. Failures retain them, while uncertain results block conflicting writes until Resolve exercise selection reconciles the saved state.

A successful switch resets only that slot: initialize weight once from the chosen exercise’s previous first set, clear reps/RIR/partials, and set technique to Decent. Other slot drafts/blockers survive. References and their late responses follow the selected identity. Continue, set limits, missing-set warnings, and completed history use the selected choice’s captured name and targets; unperformed alternatives are not missing work.

Home offers Continue session. Refresh, reopening, and database/stack restarts retain the snapshot and successfully saved sets. Continue later returns home without ending it. Unsaved browser inputs do not persist across navigation/refresh.

Cancel session confirms that the workout and **all its sets will be discarded**; Keep session/Escape leaves them intact. Completion preserves the snapshot and saved sets, records completion once, releases the active slot, and returns home. Active sets are not completed history until completion; targets, reference values, and unsaved inputs never become logged results automatically. Failed actions offer retry; stale requests cannot end a newer workout or cancel a completed one.

### Reference tabs

Each exercise card has Previous session (initial tab) and Personal best, with plain Weight (kg), Reps, RIR, and Technique tables. Repeated source occurrences stay separate and ordered; repeated current occurrences may share a reference. Empty history and retryable load failures have concise states. References load independently and never remount/reset entry controls.

- **Previous session:** latest completed workout with at least one nondeleted set for that exercise identity, across any template. Skip active/cancelled workouts and completed workouts without eligible sets. Completion time, then numeric session ID, breaks recency ties.
- **Personal best:** choose the completed workout containing the exercise's heaviest nondeleted set, with reps breaking equal-weight ties, then technique (Good > Decent > Bad), then deterministic source ordering for remaining ties. Show **all surviving sets for the exercise in that workout**, not just the winning set. This is not a volume score.

References are read-only. Renaming does not disconnect history. No automatic progression or per-set suggestion mapping is implemented.

### Add, edit, delete, and reorder sets

Below the planned set count, the Add form shows Weight, Reps, optional recorded RIR, technique, and optional partial reps. Weight is kilograms, nonnegative with at most two decimal places, including zero; reps are positive whole numbers. Recorded RIR is an optional nonnegative decimal, including zero; blank means absent and clears it on Save. It is separate from target RIR and does not affect personal-best selection. Missing recorded RIR displays “—”.

Optional partial reps are a separate whole-number count per set. Blank or zero means none; negative/fractional values are invalid. Full reps remain positive and required, so partial-only sets are not supported. Add/Edit can record or clear partials; successful Add clears them while failed saves retain them. Existing sets remain without partials. Saved sets, both history tabs, completed detail, and previous/personal-best references show a separate partial-rep label beside the full-rep value, without summing them or adding a fifth table column.

Every set requires Good, Decent, or Bad technique. New entry defaults to Decent and resets to Decent after successful Add, alongside clearing reps/RIR while retaining weight. Edit loads the saved rating; failed saves retain the chosen rating. Migration assigns Decent to all existing sets, including completed and deleted records, without changing their other data. This is a default backfill, not a claim about observed historical form. Technique selection alone, like retained weight, does not create an unfinished set.

Weight initializes **once** from the first set of the first Previous session occurrence, or stays blank without history. User edits prevent a late load from overwriting it. It never advances to another previous set. Successful Add set clears reps/RIR and retains the current weight exactly; failure retains all inputs. Tab switches and reference retries do not replace weight.

Saved rows use a larger Set N at top left and short Edit/Delete buttons at right, followed by weight/reps/RIR/technique values. Dividers separate rows. Edit opens a popup with Save/Cancel/Escape and focus restoration; failed or uncertain saves retain input and require retry before discarding the edit. There is no Discard set action. Clear unfinished reps/RIR/partials manually; resolve uncertain additions by retrying the same request, then Delete the confirmed set if necessary.

Drag saved sets with the same background/touch/keyboard behavior as template cards. Reordering stays within an occurrence and saves immediately, preserving exact values. Failure retains the desired order with Retry set order and blocks further changes/completion until resolved or refreshed. All views and comparisons use persisted set order.

New sets cannot exceed the occurrence's **captured planned count**. At the cap, the Add inputs/button are replaced by “Exercise complete”; saved sets remain editable, reorderable, and deletable. Deleting a saved set frees a slot and restores the Add form with its retained weight and normal entry defaults. Uncertain additions keep their retry controls until resolved. Retries and edits at the cap remain valid; legacy excess sets are preserved.

If another tab fills the planned count, a rejected addition shows the set-count message and Reload saved sets instead of an impossible Add retry. Inputs stay until that explicit action succeeds. Reload updates only this exercise’s saved sets, clears its rejected addition and unsaved reps/RIR/partials, resets technique to Decent, and keeps weight and other exercise drafts. Failed reloads retain inputs for retry. Uncertain saves still retry their original UUID.

### Complete with missing sets

Completion waits for pending requests, open edits, failed/uncertain changes, or unfinished reps/RIR/partials. A retained/default weight alone is not an unfinished set. Complete first refreshes the saved active snapshot. If planned sets are missing, a warning names each exercise occurrence and missing set numbers/ranges. Return/Escape keeps it active; **Complete anyway** retains the current partial or empty workout. A failed check never silently completes. A full plan completes directly. The backend permits partial/empty completion; the warning is a UI confirmation.

## Workout history

History is read-only and excludes active/cancelled workouts and deleted sets. Sets is first/default at `/history`; missing/unknown `tab` selects it, while `tab=sessions` selects Sessions. Selection and applied criteria survive direct links, refresh, Back/Forward, and keyboard tab navigation.

Sets groups matching rows by completed workout and then source occurrence, newest workout first. Captured exercise names and exact set tables are prominent above the smaller workout name and browser-local completion date on one wrapping line. Workout names open `/history?tab=sessions&session=<completed-session-id>`, clearing unrelated criteria and showing the entire selected workout rather than only the previously filtered exercise. Repeated workouts/occurrences remain separate; filters never create empty groups.

Sessions shows captured workout name/date and the full ordered exercise list. Each padded exercise row independently expands to recorded sets first and compact saved targets second; multiple exercises can remain open. No occurrence-number prefixes or green set boxes are shown. Both tabs use plain Weight (kg), Reps, RIR, and Technique tables, pointer cursors for enabled buttons/links/accordion triggers, useful empty/error/retry states, and snapshot names rather than mutable library names. Empty completed workouts appear in Sessions but contribute no Sets rows. Direct completed detail remains available; absent/foreign IDs show not found.

### Search and filters

Search is a literal case-insensitive substring of captured workout/exercise names. Sets uses one exercise identity filter; Sessions uses one template identity filter. Shared inclusive browser-local From/To dates combine with search and the active identity using AND. Historical options retain removed exercise IDs and archived/renamed template IDs, labeled with their latest captured names.

Apply/Search or Enter updates URL criteria; form drafts do not filter immediately. Tab switches retain shared criteria and both identity selections, but only the current tab's identity affects results. Clear removes criteria while retaining the tab. Blank/default values are omitted. Invalid criteria block results until corrected/cleared; valid unknown identity IDs produce no matches. Dates use the browser timezone and respect daylight-saving boundaries.

The exact completed-workout `session` selector combines with ordinary Sessions criteria. A banner offers Clear workout filter, which removes only that selector and stays on Sessions. Apply, refresh, and Back/Forward preserve it. Tab changes retain it in the URL, but Sets ignores it. Invalid/repeated selectors show validation; missing, foreign, active, or cancelled IDs yield no matching completed workout.

### Weight, rep, and technique changes

Both tabs show small signed green increases/red decreases beside weight/reps. Equal values or no matching baseline show no indicator. Text/accessible labels communicate direction independently of color. RIR is not compared. Technique has its own column: the rating text is green with an up arrow when improved, red with a down arrow when worse, and neutral when unchanged or without a baseline. Good > Decent > Bad; technique is compared independently even if weight/reps are unchanged. Accessible text names the old and new ratings. It does not multiply weight/reps or change their differences. Saved display order never sorts by quality.

Compare each surviving numbered set to the matching occurrence/set in the **latest earlier completed workout containing the exercise**, across templates, ordered by completion time then numeric session ID. Match repeated exercises by occurrence order. Missing occurrences/sets or an empty prior workout have no baseline. Unlike Previous session, this selection does **not** skip an earlier workout merely because its occurrence has no saved sets. Current filters never change the baseline, and comparisons use exact recorded numbers. Partial counts do not enter weight/rep/technique comparisons, personal-best or source-set ranking, chart values/eligibility, or any progress score. They have no separate change indicator.

### Exercise progress chart

The Sets tab shows a compact shadcn/Recharts chart only when one valid exercise identity is selected. It uses the same applied URL search/exercise/local-date criteria as the recorded sets below, including refresh and Back/Forward. Sessions and unfiltered Sets have no chart.

The metric is **Average estimated strength** per completed workout: calculate `entered weight × (1 + full reps / 30)` for every matching saved set of the exercise, then take the arithmetic mean across all its occurrences. Every positive full-rep count qualifies, including one rep; zero-weight sets contribute zero and remain in the denominator. There is no three-set requirement: incomplete workouts use their saved sets and display the contributing count. Technique and partials are context only. For 30 kg at 10, 9, and 8 reps, the set estimates are 40, 39, and 38 kg, giving a 39 kg point.

All entered weights are treated as ordinary resistance for this initial metric, including bodyweight and assisted exercises. Bodyweight/assistance are not adjusted: zero entered load produces zero, and increased assistance can increase the estimate. Label it as a recorded-load estimate, not measured strength gain. Personal-best reference selection remains unchanged.

Points follow completion time and numeric workout ID, oldest first, preserving separate workouts on the same day or exact timestamp. Tooltips show the average, saved-set count, captured workout, browser-local completion date/time, and contributing sets with exact weight/full reps, source occurrence/set, technique, optional RIR, and partials. Touch and keyboard arrow navigation inspect points; the existing recorded-set list below provides source data without an additional expandable chart-data list. A readable latest-average/count summary complements the chart. The chart has no animation and uses light/dark tokens on phone/desktop.

Zero matching workouts show an explanation; one workout shows a visible point with no trend claim. Loading, retryable failure, and unsupported-metric states remain independent of usable recorded sets. Old or aborted requests cannot replace newer filter results. Renames, template edits/archives, and deletion of an unused exercise preserve captured chart data.

## Temporary testing data

The optional Home reset is disabled by default and requires explicit enablement and confirmation. It atomically replaces the local owner's workout data within the existing schema, preserving migration/owner records and increasing IDs. Failure rolls back; success refreshes continuation. The dataset has 26 tagged/described exercises, four described Upper/Lower templates with approved occurrence order/counts, 48 completed workouts over 12 weeks (1,032 sets), and one active workout with three saved sets. All four routines offer variations using existing library exercises, including presses, pull-ups, curls, triceps exercises, and squat/leg-extension choices. Upper 1’s preacher-curl slot offers two alternatives. Historical workouts rotate the actual choices, preserving each slot’s planned set count and all dataset totals; the active example starts with defaults. Optional target rep ranges/RIR remain unset; pipes do not imply supersets. Actual RIR defaults to absent. A deterministic mix of dummy sets has positive partial counts or none, without changing their full reps, weights, or technique. Technique cycles deterministically through Good, Decent, and Bad, including historical improvements, regressions, unchanged ratings, and paired weeks with equal weight/reps to exercise tie-breaking. [README](../README.md#temporary-dummy-data-reset) explains operation and data care.

## Future work and unresolved choices

Authentication/multiple users, custom muscle groups, warm-up/individual-set targets, planned weights, timers/supersets, imports/integrations, offline synchronization, permanent template deletion, and corrections/deletion of completed workouts require separate scope. Do not treat previous proposed schemas, pagination, replay protocols, or progression rules as approved behavior.
