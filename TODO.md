# TODO

Holding pen for the wishlist that used to live at the bottom of `README.md`.
Items here have not been triaged; some will graduate to GitHub Issues, some
will be deleted as obsolete, some will become plan documents under
`docs/plans/active/` when their time comes. Add new wishlist items here
freely; the README itself stays a marketing/getting-started doc.

## Todo

- [ ] Dedup - Find duplicate and similar tasks. See dedup example
- Interactive Mode
  - [ ] Interact: implement pager for lists
  - [ ] Interactive: Enable task edit
  - [ ] Interact: Move cursor with current task when changing priority
  - [ ] Interact: Enable task highlighting by priority (Maybe just priority?)
  - [ ] Interact: Highlight tags KV and dates in task render
  - [ ] Interact: Add `created-on` tag for new tasks
  - [ ] Interact: fix help menu column balance issue
  - [ ] Enable command mode
- QA
  - [ ] Write intergation tests for htt. Make sure that mutating actions work as intended
- Repos
  - [ ] Check if repo exists. If it doesn't clone it first
  - [ ] Handle case when local repo is behind remote. (Rebase? Merge? How?)
  - [ ] Add 'status' action which shows sync status
  - [ ] Commit on all modifications.
- UX
  - [ ] Enable managing contexts (archive, delete, merge)
  - [ ] Add `due:Friday` smart parsing
  - [ ] Add `due:"in two weeks"` smart parsing
  - [ ] Enable showing in priority order
  - [ ] Add deduplicate command (smart with edit distance?)
  - [ ] Add "log@" command to log at a particular time
  - [ ] Review jrnl command to see if we can take inspiration from this
  - [ ] Add "Where is <>?" command
  - [ ] archive context
  - [ ] archive todo
  - [ ] Query language
  - [ ] interval output
  - [ ] set up "htt do context/line" command to complete tasks across contexts
- [ ] Fork goparsec to fix messed up API choices
  - AST/Node distinction?
  - Simple querying

## Production grade

- [ ] Set up packaging for release. (see https://github.com/goreleaser/goreleaser)
- [ ] Set up CI

## Future

- [ ] Set up interactive cli (see https://github.com/c-bata/go-prompt)
- UI. Either:
  - [ ] Set up a CLI GUI like todotxt-machine (using https://github.com/jroimartin/gocui or https://github.com/gizak/termui)
  - [ ] Delegate full editing to other program (https://github.com/AnthonyDiGirolamo/todotxt-machine)
- [ ] Create a GUI app layer (https://github.com/avelino/awesome-go#gui, https://github.com/murlokswarm/app, )

## Done

- [x] Interactive: Add new task
- [x] Interactive: Manage priorities
- [x] Interactive: autosort task list
- [x] Set up interactive mode with Bubbletea, bubbles and lipgloss
- [x] Add "summary" command to show tasks and prioritoes across contexts
- [x] Action: increase priority
- [x] Action: decrease priority
- [x] Add 'do' action
- [x] Add 'Add to' action
- [x] Add 'switch' action (context alias)
- [x] Enable short commands
- [x] Add 'work on' action
- [x] Add 'add+workon' action ('log' ?)
- [x] Show active and other contexts on `show`
- [x] Create a todotxt line parser (https://github.com/todotxt/todo.txt)
  - https://github.com/alecthomas/participle
  - https://github.com/mna/pigeon
  - https://github.com/prataprc/goparsec
  - https://github.com/vektah/goparsify
  - https://github.com/pointlander/peg
