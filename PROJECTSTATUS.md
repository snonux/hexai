# Ideas

## Code qualoty

* [/] TODO's in the code to be addressed
* [/] No more than 1000 LOC per source file
* [/] No more than 50 LOC per function
* [/] Each struct type in his own file
* [/] Sufficient unit tests

## Features

### Improvements

* [X] Include unit test coverage reports
* [ ] Change inline triggers to include > to be more consistent with other triggers

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

## Usage notes
