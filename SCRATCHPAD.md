# Project scratch pad

This document shows future items and items in progress. Already completed ones are deleted from this document as updates occur.

## Features

* [ ] Keep global stats about LLM usage for the tmux pane!
* [ ] No a feature, but verify my OpenAI API account so I can use GPT-5 via the API.
* [ ] In-editor chat triggers should be context aware of the current file, buffer and function!
* [ ] Kagi FastGPT for in-editor search
  - Think about an in-editor chat trigger, maybe with S> for search!
* [ ] Test whethe GitHub Copilot support actually works now, and if not, fix it!
* [ ] Be able to re-configure the temperature in-editor
* [ ] For in-editor chat add a way to print current hexai status such as
  * active LLM
  * active stats
  * current config printed out
  * could use a special keyword like /status?TRIGGER (e.g. > as TRIGGER)
  * what other slasm commands could we think of?
* [ ] Be able to switch LLMs ad-hoc.

## More

* [ ] Configure hexai-lsp multiple times, but with different LLM backends? E.g. a remote cloud one and a local one?
* [/] Review documentation
* [/] Manual review the code 
* [ ] Useful: https://deepwiki.com/helix-editor/helix/4.3-language-server-protocol

## Additional Ideas (Nice to Have)

*   **LSP: Inlay Hints (`textDocument/inlayHint`)**: Use AI to provide dynamic, inline hints like inferred types, performance warnings (e.g., `O(n^2)`), or security alerts.
*   **LSP: Hover (`textDocument/hover`)**: Enrich hover popups with AI-generated natural language explanations of code, usage examples, and complexity analysis.
*   **LSP: Semantic Tokens (`textDocument/semanticTokens`)**: Implement semantic tokenization to build a deeper understanding of the code, which would improve the accuracy and context-awareness of all other AI features.
*   **LSP: Rename (`textDocument/rename`)**: Add safe rename capabilities, potentially enhanced with AI to proactively suggest better, more descriptive names for variables and functions.
