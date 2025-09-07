# Repository Guidelines

## Project Structure & Module Organization

- `README.md`: Project overview and quick context.
- `assets/`: Images and brand assets
- `scripts/`: Helper tools and maintenance scripts.

## Coding Style & Naming Conventions

- Avoid duplication of code when the functions are larger than 5 lines.
- If possible, construct individual methods so that they can be unit tested. But only if it doesn't add too much boilerplate to the code base.
- Aim for at least 85% unit test coverage of all source code. The command to check the coverage is "mage coverage"
- Ensure that all unit tests pass before commiting any changes.
- Always run the gofumpt code reformatter on all go files modified.
- There should be no source code file larger than 1000 lines. If so, split it up into multiple.
- There should be no function larger then 50 lines. If so, refactor or split up into multiple smaller functions.
- Filenames: docs use `lowercase-with-dashes.md`; images use kebab‑case with size/purpose suffix (e.g., `hexai-small.png`).
- Code (when added): follow language idioms
- Any type with more than 3 methods should be in it's own source code file, whereas the filename contains the name of the type.
