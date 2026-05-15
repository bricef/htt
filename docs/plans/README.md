# Plans

Long-form design documents for non-trivial work. Each plan captures the
goal, the decisions taken (with their rationale), the step-by-step
sequence, and the test harness for each step.

## Layout

- `active/` — plans whose work is in progress. A plan lives here from
  the moment its design dialogue is captured through to the merge of
  its final step. The plan's `**Status:**` header reads `Proposed` or
  `In progress`.
- `complete/` — plans whose work has landed on `main`. The plan's
  `**Status:**` header reads `Complete (YYYY-MM-DD)`. Plans stay here
  as the historical record of why decisions were made.

A plan moves from `active/` to `complete/` when its final step is
merged. Don't delete completed plans — they're cheap to keep and
they're the answer to "why is the codebase shaped this way".

## Naming

`YYYY-MM-DD-short-slug.md` where the date is the day the plan started.
The date helps order them chronologically when listed.
