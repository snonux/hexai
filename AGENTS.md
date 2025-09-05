# Repository Guidelines

## Project Structure & Module Organization

- `README.md`: Project overview and quick context.
- `assets/`: Optimized images and brand assets (place new images here). Existing
  legacy files: `hexai.png`, `hexai-small.png`.
- `src/`: Future implementation code.
- `tests/`: Future test suites mirroring `src/` paths.
- `scripts/`: Helper tools and maintenance scripts.

## Build, Test, and Development Commands

- Lint Markdown: `markdownlint **/*.md` — checks heading/style rules.
- Spellcheck: `codespell` — catches common typos.
- Optimize images: `pngquant --quality=70-85 input.png -o assets/input.png`.
- No build step required for docs-only changes.

## Coding Style & Naming Conventions

- Aim for at least 80% unit test coverage of all source code.
- Ensure that all unit tests pass before merging any changes.
- If possible, construct individual methods so that they can be unit tested. But only if it doesn't add too much boilerplate to the code base.
- There should be no source code file larger than 1000 lines. If so, split it up into multiple.
- There should be no function larger then 50 lines. If so, refactor or split up into multiple smaller functions.
- Markdown: ATX `#` headings, sentence‑case titles, wrap lines ~100 chars,
  use fenced code blocks and descriptive link text.
- Filenames: docs use `lowercase-with-dashes.md`; images use kebab‑case with
  size/purpose suffix (e.g., `hexai-small.png`).
- Code (when added): follow language idioms
