# Ideas

## Code quality

### Refactor

* [ ] Refactor existing code in a more modular way
* [ ] Add unit tests

## Features

### Improvements

* [ ] TODO's in the code to be addressed

### New features

* [ ] Resolve diagnostics code action feature
* [X] LSP server to be used with the Helix text editor
* [X] Code completion using LLMs
* [X] Text completion in general
* [/] Code generation using LLMs text
* [ ] Be a replacement for 'github copilot cli'
* [ ] Be able to perform inline chats (keeping history in the document)
* [ ] Be able to switch the underlying model via a prompt
* [ ] Fine tune when LLM completions are triggered, as it seems that there are some cases where the LLM is asked but Helix is not suggesting any completions

Be able to select code blocks and perform code actions on them

* [ ] Commenting exiting code
* [ ] Code refactoring

Be able to chat with the LLM

* [ ] Have a dialog with the LLM, like in lsp-ai

Be able to switch LLMs. 

* [ ] Ollama local LLM models (e.g. Qwen Coder vs Deepseek-R1 for different purposes)
* [ ] OpenAI models
* [ ] Claude models
* [ ] Gemini models

## More

* [ ] Useful: https://deepwiki.com/helix-editor/helix/4.3-language-server-protocol` 

## Usage notes

Helix' `languages.toml`

```toml
[[language]]
name = "go"
auto-format= true
diagnostic-severity = "hint"
formatter = { command = "goimports" }
language-servers = [ "gopls", "golangci-lint-lsp", "hexai" ]
# language-servers = [ "gopls", "golangci-lint-lsp", "lsp-ai", "gpt", "hexai" ]

[language-server.hexai]
command = "hexai"
`

## Prompting

* Write a new function: `;Implement a function counting the number of files in a directory;`
* In-place code add: `;Get number of files in a directory;`
