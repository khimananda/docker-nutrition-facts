# Contributing

## The easiest PR: add a footnote

Open `internal/phrases/footnotes.txt` and add one line. Rules:

- 44 characters or fewer. A test enforces it, because the label has a fixed width.
- Kind. Punch at the image, never at people or projects.
- Plausible on a food label. "Keep out of reach of production." works. A paragraph does not.

Run `go test ./...` and open the PR.

## A new fact or heuristic

Heuristics live in `internal/facts/heuristics.go`, one small function each, with a table test in `heuristics_test.go`. They only ever see image metadata: history lines, env var names, the user, sizes. Never read or print env var values.

When a heuristic changes the label, regenerate the golden files:

```
go test ./internal/render/ -update
git diff internal/render/testdata
```

Read the diff. The golden files are the spec.

## Grades

Scoring is in `score()` in `internal/facts/report.go`. Every deduction must appear in the "Why this grade" list, so people can see what to fix. If you change the weights, run the tool on a few popular images and put the before and after grades in the PR.
