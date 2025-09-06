# Project status

This documents shows future items and in progress items. Already completed ones are removed from this document as we go.

## Features

### AI menu

```
[keys.normal]
C-p = ":sh hexai --menu"
```

Or maybe

```
[keys.normal]
C-p = ":sh hexai-action"
```


And then generate a menu with all the code actions hexai-lsp knows of and include hotkeys for each menu item! Also print out a notice that this is a work-around due to limitations in Helix's current LSP UI.

### More features

* [ ] Have all text LLM prompts be configurable. With defaults as of now.
* [ ] implement a code action for selected code block the way via a unix pipe as faster access in helix
  - pipe selected code to external command and replace selection with output
  - the external command should open a menu to select an action (e.g. "format", "refactor", "explain", "test", etc.) and then apply it to the selected code
  - the external menu can be opened in a separate tmux pane
* [ ] Fish and zsh and bash shell integration for command completion? Have access to the current shell history (N last commands), current directory content, and current directory name.
* [ ] Be able to re-configure the temperature in-editor
* [ ] Unit test config loading

Be able to switch LLMs. 

* [ ] Ollama local LLM models (e.g. Qwen Coder vs Deepseek-R1 for different purposes)
* [ ] OpenAI models
* [ ] Claude models
* [ ] Gemini models

## More

* [ ] Useful: https://deepwiki.com/helix-editor/helix/4.3-language-server-protocol` 

Can Claude Code be used for Code Completion?

> Yes, Claude (including Claude's coding capabilities, sometimes referred to as "Claude Code") can be used for code completion. You can provide Claude with a partial code snippet or function, and it can generate the next lines or complete the function based on context and intent. This works similarly to how other AI code assistants (like GitHub Copilot or ChatGPT) provide code completion.

However, Claude is typically accessed via chat or API, rather than being directly integrated into IDEs for real-time, inline code completion. Some third-party tools or plugins may bridge this gap, but out of the box, Claude is best used for code completion via conversational prompts.

**In summary:**  
- Claude can perform code completion when prompted.
- It is not natively integrated into IDEs for real-time completion, but can be used via chat or API.
- You can use it to finish functions, suggest code, or generate boilerplate based on your input.

j
