---
name: /opsx-verify
id: opsx-verify
category: Workflow
description: Thin Harness adapter for running repository workflow verification
---

This command is a thin Harness adapter.

Repo-level workflow truth lives in:

- `HARNESS.md`
- `harness/workflows/verify.md`

Before acting:

1. Read `HARNESS.md`
2. Run `./harness/bin/check`
3. Read `harness/workflows/verify.md`
4. Use `./harness/bin/opsx verify`

Do not maintain a second copy of the workflow here. Follow the matching commands from `harness/workflows/verify.md`.
