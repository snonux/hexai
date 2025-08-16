# Repository Guidelines

## Project Structure & Module Organization

- `README.md`: Project overview and quick context.
- `IDEAS.md`: Working notes, concepts, and rough drafts.
- `assets/`: Optimized images and brand assets (place new images here). Existing
  legacy files: `hexai.png`, `hexai-small.png`.
- `src/`: Future implementation code.
- `tests/`: Future test suites mirroring `src/` paths.
- `scripts/`: Helper tools and maintenance scripts.

## Build, Test, and Development Commands

- Preview Markdown: `glow README.md` (or your editor’s preview).
- Lint Markdown: `markdownlint **/*.md` — checks heading/style rules.
- Spellcheck: `codespell` — catches common typos.
- Optimize images: `pngquant --quality=70-85 input.png -o assets/input.png`.
- No build step required for docs-only changes.

## Coding Style & Naming Conventions

- There should be no source code file larger than 1000 lines. If so, split it up into multiple.
- There should be no function larger then 50 lines. If so, refactor or split up into multiple smaller functions.
- Markdown: ATX `#` headings, sentence‑case titles, wrap lines ~100 chars,
  use fenced code blocks and descriptive link text.
- Filenames: docs use `lowercase-with-dashes.md`; images use kebab‑case with
  size/purpose suffix (e.g., `hexai-small.png`).
- Code (when added): follow language idioms; use consistent 2 or 4‑space
  indentation; avoid one‑letter identifiers; keep functions short and focused.
