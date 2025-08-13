# Repository Guidelines

This repository currently holds documentation and brand assets for HexAI. It is intentionally lightweight; additional modules and code may be added over time. The guidance below keeps contributions consistent and easy to review.

## Project Structure & Module Organization
- `README.md`: Project overview and quick context.
- `IDEAS.md`: Working notes, concepts, and rough drafts.
- Images: `hexai.png`, `hexai-small.png` (place new images under `assets/` going forward, referenced with relative paths).
- Future code (if added): `src/` for implementation, `tests/` for test suites, `scripts/` for helper tools.

## Build, Test, and Development Commands
- No build step required for docs-only changes.
- Preview Markdown: use your editor’s preview or `glow README.md`.
- Optional checks (if installed locally):
  - `markdownlint **/*.md`: Lint Markdown formatting.
  - `codespell`: Catch common typos.
- Optimize images before committing, e.g.: `pngquant --quality=70-85 input.png -o assets/input.png`.

## Coding Style & Naming Conventions
- Markdown: ATX `#` headings, sentence-case titles, wrap lines near ~100 chars, use fenced code blocks and descriptive link text.
- Filenames: lowercase-with-dashes for docs (e.g., `design-notes.md`); images: kebab-case with size or purpose suffix (e.g., `hexai-small.png`).
- If/when code is added: follow language idioms, 2 spaces or 4 spaces consistently per language, avoid one-letter identifiers, and keep functions short and focused.

## Testing Guidelines
- For now: validate links render and assets load; run a Markdown linter locally if available.
- When tests exist: place unit tests in `tests/` mirroring module paths; name tests `test_<module_or_feature>.ext`; target high-value paths first.

## Commit & Pull Request Guidelines
- History is currently informal; adopt Conventional Commits (e.g., `feat:`, `fix:`, `docs:`) going forward.
- Commits: small, scoped, and imperative subject line (≤72 chars).
- PRs: clear description, link related issues, include before/after screenshots for visual or asset changes, and note any follow-ups.

## Security & Asset Tips
- Do not commit secrets or credentials.
- Keep binary assets lean (<5 MB preferred); compress images and remove unused files.

