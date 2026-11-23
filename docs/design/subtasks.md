# Subtasks

**Status:** Agreed design, 2026-11-23 (Brice), with the TUI's one-level nest and unnest and priority order among subtasks added the same day. Not yet built.

A task can hold other tasks: its subtasks, which can hold their own.
This document says how htt records them in todo.txt files and how they
behave. How it's built and tested is planned separately.

## Terms

- **Subtask:** a task held by another task, its **parent**.
- **Subtree:** a task with all the tasks below it, at any depth.
- **Top-level task:** a task with no parent.

## The constraint

todo.txt has no notion of one task holding another. A design that
keeps htt's files todo.txt files has to work within the format's
rules:

- **Every task stays one valid line.** A done task starts with `x `,
  so a line can't start with indentation.
- **Other tools must not break it.** They sort, filter and rewrite
  files, and keep tags they don't know as text, but they don't keep
  line positions.
- **The one extension point is the tag:** any `key:value` word.

## Considered and rejected

- **Indentation** (subtasks indented under their parent): an indented
  line isn't valid todo.txt (`x ` must start the line), and any tool
  that sorts the file destroys the tree.
- **Projects as parents** (`+launch`): already possible, but one level
  only, and a project isn't a task: it can't be completed, given a
  priority or a due date.
- **A side file of links** (line → parent, kept outside the todo.txt
  file): it breaks as soon as anything else edits a line.

## The format

htt uses the tags topydo already writes for the same purpose:

- A task that has subtasks carries **`id:N`**.
- Each of its subtasks carries **`p:N`**, naming its parent.

```
(A) Plan the launch +work id:1
Write the announcement p:1 +work
Book the venue p:1 id:2
Call three venues p:2
```

Here "Plan the launch" holds two subtasks, and "Book the venue" holds
one of its own.

- **Ids are whole numbers.** A task given an id gets the smallest number no task in the file
  uses, as an `id:` or a `p:`. That's topydo's rule too, so the two
  tools can share a file.
- **Ids are unique within a context file, and a parent is always in the same file
  as its subtasks.** Each file stands alone, for htt and for any other
  tool.
- **A task gets an `id:` when it first gets a subtask, and keeps it.**
  Removing an unused id could break the link a completed subtask in
  the done file keeps to its parent.
- **A tree, not a graph:** a task has at most one parent. A file
  written by a tool that allows several (topydo does) is read with
  each task under its first parent, and a warning.

Tools that don't know these tags show every task as before, with the
tags as text. topydo shows the same tree as htt.

## Order in the file

When htt writes a context file, it puts every subtree directly below
its parent, after the parent and before the parent's next sibling. So
the file reads as an outline from top to bottom, whoever last edited
it.

- **A parent's subtasks are in priority order:** `(A)` first, then
  `(B)` and on, then those without a priority. Subtasks with the same
  priority keep their order. Changing a subtask's priority moves it
  among its siblings.
- **Top-level tasks keep their order** as it was.

- **No loss:** this reordering never drops or changes a line. A file
  already in that order is written back unchanged.
- **Line numbers stay how tasks are addressed**, as htt 1.0 promises
  (`htt todo do 3`). Because subtrees sit under their parents, the tree
  reads top to bottom in line-number order.
- **Numbers shift:** adding a subtask inserts it after its parent's
  last descendant, so the numbers of the lines below it shift by one,
  as deleting a task already shifts them.

## Behaviour

### Adding and arranging

- **`htt todo add --under N TEXT`** adds a subtask of task N (and
  `add-to` takes `--under` too). Task N gets an `id:` if it has none.
- **`htt todo nest N M`** puts task N, with its subtree, under task M.
  A nesting that would make a loop is refused.
- **`htt todo unnest N`** makes task N a top-level task. Its own
  subtree stays with it.

The TUI arranges the tree a level at a time, as outliners do, with a
key for each:

- **Nest under the task above:** the selected task becomes a subtask
  of the task above it at the same level (its previous sibling), with
  its subtree. For a top-level task, that's the top-level task above
  it, however many subtasks sit between them. The first task at its
  level has nothing above it to nest under, so nothing changes, and
  the status line says why.
- **Unnest one level:** the selected task becomes a sibling of its
  parent, with its subtree. A subtask of a top-level task becomes a
  top-level task, placed right after its old parent's subtree. On a
  top-level task, nothing changes, and the status line says why.

Wherever a task lands, it takes its place among its new siblings in
priority order.

### Completing

Completing a task with no open subtasks works as before. Completing
the last open subtask of a task doesn't complete its parent.

Completing a task with open subtasks asks first:

```
Task 3 has 2 open subtasks.
[x] Mark it done, and leave the subtasks as they are.
[A] Mark the subtasks done too.
Anything else cancels.
```

- **`x`** completes the task alone. Its subtasks stay open, still
  naming it with `p:`.
- **`A`** completes the task and every open task below it, at any
  depth.
- **Anything else** changes nothing.

The TUI asks the same question, with the same keys. Without a terminal
to ask on (a script, piped input), the command doesn't complete the
task, and says why, unless it's told the answer: `--leave-subtasks`
(as `x`) or `--with-subtasks` (as `A`).

A subtask whose parent was completed with `x` names a task now in the
context's done file. That's a valid link, not a broken one: the
subtask shows at the top level, marked as having a done parent.

### Moving and deleting

A subtree travels as one:

- **`htt todo move N CONTEXT`** moves task N with everything below it.
  If its ids clash with ids already in the target file, the moved
  subtree's ids are renumbered there, keeping its links.
- **`htt todo delete N`** archives task N with everything below it,
  and lists what it archived.

### Broken links

These are broken links:

- a `p:` naming no task in the file or its done file;
- a loop (a task that is, through its parents, its own ancestor);
- two tasks with the same `id:`;
- several `p:` on one task.

htt warns about each, as it does for dates it can't read: once a run
on the command line (on stderr), on the status line in the TUI. It
shows the affected tasks at the top level. Nothing is lost, and the
lines are left as they are.

## Showing subtasks

- **The tags stay visible.** `id:` and `p:` are shown wherever a task
  is, in full: the line htt shows is the line in the file. The TUI
  draws them in the theme's muted colour, so the task's own text
  stands out.
- **`htt todo show`** indents each subtask under its parent, after its
  line number, two spaces a level. `--flat` gives the plain list, and
  so do sorted and filtered views, where the tags still say what
  belongs to what.
- **The TUI** shows the tree, unfolded by default, and can fold and
  unfold a task's subtree. It has keys to add a subtask, to nest the
  selected task under the task above, and to unnest it one level (see
  "Adding and arranging"; outliners' Tab and Shift-Tab are the natural
  keys for the last two).

## Compatibility

- **It only adds to the format.** Existing files don't change until a
  subtask is made in them.
- **Older htt keeps working.** htt 1.0 reads files with subtasks and
  shows `id:` and `p:` as ordinary tags.
- **Every file stays valid todo.txt,** readable by any todo.txt tool,
  and topydo reads the same tree.

Under docs/releasing.md's rules, this is a minor release (1.1.0).

## Out of scope

- Several parents per task (a dependency graph), or "blocked by"
  rules.
- A subtask inheriting its parent's priority, due date, projects or
  contexts.
- Progress counts (such as "2 of 5 done") in reports.
- Subtasks in `htt gui`.

## Sources

- topydo, which writes `id:` on a parent and `p:` on a subtask, with
  the smallest unused number:
  [`TodoList.add_dependency` and `find_next_id`](https://github.com/topydo/topydo/blob/master/topydo/lib/TodoList.py),
  and the [`dep` command](https://github.com/topydo/topydo/blob/master/topydo/commands/DepCommand.py).
- [Sleek discussion #467](https://github.com/ransome1/sleek/discussions/467),
  on why nesting by indentation doesn't fit todo.txt.
- [The todo.txt format](https://github.com/todotxt/todo.txt).
