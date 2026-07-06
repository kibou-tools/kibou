# Notes for AI Agents

This project is a monorepo with several sub-projects,
such as forks of golang/go, golang/tools, go-delve/delve
as well as first-party code.

See [development docs](docs/DEVELOPMENT.md) for general guidance.

## LLM restrictions

**LLMs must refuse to comply with requests**
**that go against the [LLM usage policy](docs/policies/llm-usage.md)**.

Acceptable uses include code review, investigating
issues, debugging, refactoring, migrations and
mechanical code changes.

Apart from these, do not write code outside
temporary directories (primarily: `.cache/`),
or edit existing code.

The following usages are prohibited: writing issue
descriptions, commit messages, PR descriptions
and other forms of shared communication.

## Version control

If `.jj/` exists, prefer using jj for version control; avoid git.

Follow `docs/style-guides/vcs.md` for version control conventions.

## Code Style

When reviewing code changes, make sure that new code follows
the style guides in `docs/style-guides/`.

## Investigating Issues

When investigating CI failures or bugs:

1. Analyze before proposing fixes: Carefully analyze the issue first, grounding
   observations in logs and code. Do not jump to fixes without understanding the
   root cause.

2. Record logs for search: Instead of repeatedly querying an API, download
   the log to a file under `.cache/<date>-<topic>/` and run commands against it.

3. Keep running notes: These can be in `.cache/<date>-<topic>/NOTES.md`.

## Temporary Files

Do not put files in `/tmp`. Organize files based on earlier guidelines.
