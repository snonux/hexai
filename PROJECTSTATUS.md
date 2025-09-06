# Ideas

## Features

### New features

* [X] Create "generate unit test" code action for selected code block => write test to FILE_test.go file
* [ ] implement a code action for selected code block the way via a unix pipe as faster access in helix
* [X] Use hexai as a gh copilot... CLI replacemant for command line questions
* [X] Resolve diagnostics code action feature
* [X] LSP server to be used with the Helix text editor
* [X] Code completion using LLMs
* [ ] Have all text LLM prompts be configurable. With defaults as of now.
* [X] Text completion in general
* [/] Be a replacement for 'github copilot cli'
* [X] Be able to perform inline chats (keeping history in the document)
* [ ] Be able to switch the underlying model via a prompt
* [X] Fine tune when Large Language Model (LLM) completions trigger, as it seems that there are some cases where the Large Language Model (LLM) receives a request but Helix isn't suggesting any completions. There seems to be something odd with the in logic. Investigate the TriggerChar logic and make sure it matches Helix's expectations.
* [X] Can anything else can be done with LSP?
* [ ] Fish and zsh and bash shell integration for command completion? Have access to the current shell history (N last commands), current directory content, and current directory name.

Be able to select code blocks and perform code actions on them

* [X] Commenting exiting code
* [X] Add unit test (for Go)
* [X] Code refactoring (via comment instruction)

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
