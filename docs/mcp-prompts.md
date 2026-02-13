# MCP Prompts Guide

> **⚠️ DEPRECATION NOTICE**
>
> This MCP server is **EXPERIMENTAL** and **NOT ACTIVELY MAINTAINED**.
>
> The author currently manages prompts through slash commands and meta-commands
> in the hexai agent system, making this MCP server redundant for its original
> purpose. This code is kept for potential future enhancements (possibly with
> different functionality beyond prompt management), but no guarantees are made
> about stability or continued support.
>
> **This documentation is preserved for reference only.**

## Overview

Prompts in hexai-mcp-server are reusable templates that can be parameterized with arguments. They're stored in JSONL format (one JSON object per line) for easy editing and version control.

## Prompt Structure

Each prompt has the following fields:

```json
{
  "name": "code_review",
  "title": "Request Code Review",
  "description": "Analyzes code quality, style, and suggests improvements",
  "arguments": [
    {
      "name": "code",
      "description": "The code to review",
      "required": true
    }
  ],
  "messages": [
    {
      "role": "user",
      "content": {
        "type": "text",
        "text": "Please review the following code:\n\n{{code}}"
      }
    }
  ],
  "tags": ["development", "review", "quality"],
  "created": "2026-02-10T12:00:00Z",
  "updated": "2026-02-10T12:00:00Z"
}
```

### Field Descriptions

- **name**: Unique identifier (alphanumeric + underscores only)
- **title**: Human-readable display name
- **description**: Brief explanation of what the prompt does
- **arguments**: List of template variables (see below)
- **messages**: Conversation messages with roles (user/assistant)
- **tags**: Array of categorization tags for filtering
- **created/updated**: ISO 8601 timestamps

### Arguments

Arguments define variables that can be substituted in message templates:

```json
{
  "name": "variable_name",
  "description": "What this variable represents",
  "required": true
}
```

- **name**: Variable name used in templates as `{{variable_name}}`
- **description**: Help text for users
- **required**: Whether the argument must be provided

### Messages

Messages define the conversation flow:

```json
{
  "role": "user",
  "content": {
    "type": "text",
    "text": "Prompt text with {{arguments}}"
  }
}
```

- **role**: Either `"user"` or `"assistant"`
- **content.type**: Currently only `"text"` is supported
- **content.text**: Message text with `{{argument}}` placeholders

## Built-in Prompts

hexai-mcp-server includes built-in **meta-prompts** that help you create and manage prompts interactively. These prompts leverage Claude's access to your conversation context to extract content and create templates.

### save_prompt
Interactively create a new prompt template from your current conversation.

**How it works**:
1) You provide a name and title for the new prompt
2) Claude analyzes your conversation to understand what should be templated
3) Claude asks clarifying questions about arguments, description, and tags
4) Claude shows a complete preview of the prompt structure
5) After you approve, Claude saves it using the MCP prompts/create method

**Arguments**:
- `prompt_name` (required): Unique identifier (lowercase, underscores allowed)
- `prompt_title` (required): Human-readable display name

**Usage Example**:
```
You: (Have a conversation about code review)
     "Can you help me review Go code for best practices?"
Claude: (Provides code review suggestions)

You: /save_prompt prompt_name="go_best_practices" prompt_title="Go Best Practices Review"

Claude: I'll help you create this prompt. Looking at our conversation, I see you want
        a code review focused on Go best practices.

        Clarifying Questions:

        1) Template Arguments
        Should the 'code' argument be required?
        a) Yes, required (can't review without code)
        b) No, optional

        2) Description
        What description would best explain this prompt's purpose?
        a) "Reviews Go code for adherence to best practices and idiomatic patterns"
        b) "Analyzes Go code quality focusing on style, performance, and Go idioms"
        c) Something else?

        3) Tags
        Which tags would help categorize it?
        a) ["go", "review", "best-practices"]
        b) ["development", "go", "code-quality"]
        c) Other suggestions?

        ...

        Here's the preview:
        ```json
        {
          "name": "go_best_practices",
          "title": "Go Best Practices Review",
          ...
        }
        ```

        Should I save this prompt?

You: Yes, looks good!

Claude: (Saves the prompt using MCP prompts/create)
```

**Tags**: meta, prompt-management, interactive

### update_prompt
Interactively modify an existing prompt.

**How it works**:
1) You provide the name of the prompt to update
2) Claude fetches and shows the current version
3) Claude asks what changes you want to make
4) If you reference conversation content, Claude helps extract and template it
5) Claude shows a preview with changes highlighted
6) After you approve, Claude updates it using the MCP prompts/update method

**Arguments**:
- `prompt_name` (required): Name of the existing prompt to update

**Usage Example**:
```
You: /update_prompt prompt_name="go_best_practices"

Claude: Let me fetch the current version of 'go_best_practices'...

        Current prompt:
        - Name: go_best_practices
        - Title: Go Best Practices Review
        - Description: Reviews Go code for idiomatic patterns
        - Arguments: code (required)

        What changes would you like to make?

You: Add an optional 'focus_area' argument that lets me specify what aspect to focus on

Claude: Great! I'll add that argument. Here's the updated preview with changes:

        ```json
        {
          "name": "go_best_practices",
          "arguments": [
            {"name": "code", "required": true, ...},
            {"name": "focus_area", "required": false, ...}  // ← NEW
          ],
          "messages": [
            {
              "text": "Review this Go code for best practices:\n\n{{code}}\n\nFocus on: {{focus_area}}"  // ← UPDATED
            }
          ]
        }
        ```

        Should I save these changes?

You: Yes!

Claude: (Updates the prompt using MCP prompts/update)
```

**Tags**: meta, prompt-management, interactive

### design_prompt
Interactively design a brand new prompt template from scratch with minimal questions.

**How it works**:
1) Claude asks what the prompt should do (1-2 sentences)
2) Claude asks what arguments it needs (if any)
3) Claude asks for metadata (name, title, tags)
4) Claude shows a preview of the complete prompt
5) After you approve, Claude saves it using the MCP prompts/create method

**Arguments**: None (fully interactive)

**Usage Example**:
```
You: /design_prompt

Claude: What should this prompt do?

You: Review database schemas and suggest optimizations for performance and best practices

Claude: What arguments does it need?

You: schema (required), db_type (optional)

Claude: Prompt name, title, and tags?

You: Name: review_db_schema
     Title: Database Schema Review
     Tags: database, schema, review

Claude: Here's your prompt:

        ```json
        {
          "name": "review_db_schema",
          "title": "Database Schema Review",
          "description": "Review database schemas and suggest optimizations for performance and best practices",
          "arguments": [
            {"name": "schema", "required": true},
            {"name": "db_type", "required": false}
          ],
          "messages": [...],
          "tags": ["database", "schema", "review"]
        }
        ```

        Should I save this?

You: Yes

Claude: (Saves the prompt using MCP prompts/create)
```

**Tags**: meta, prompt-management, interactive, creation

**Note**: Built-in prompts (including these meta-prompts) cannot be modified or deleted. If you need to customize a built-in, create a new prompt with a different name.

## Creating Custom Prompts

### Storage Files

Prompts are stored as follows:
- **Built-in prompts**: Compiled into the `hexai-mcp-server` binary (no file needed)
- **User prompts**: Stored in `user.jsonl` at `~/.local/share/hexai/prompts/` (or your configured directory)

This means built-in prompts are always available and up-to-date with the binary version.

### Method 1: Interactive Meta-Prompts (Recommended)

Use the built-in `save_prompt` meta-prompt to create prompts interactively. This is the easiest method because:
- Claude extracts content from your current conversation
- You get guided questions about templating
- You see a preview before saving
- No manual JSON editing required

**Example workflow**:
```
1) Have a conversation with Claude about your task
   You: "Help me write better commit messages"
   Claude: (Provides guidance on commit message best practices)

2) Invoke the meta-prompt
   You: /save_prompt prompt_name="better_commits" prompt_title="Better Commit Messages"

3) Answer Claude's clarifying questions
   Claude: "Should I template the 'diff' as an argument?"
   You: "Yes, make it required"

4) Review and approve
   Claude: (Shows JSON preview)
   You: "Looks good!"

5) Prompt is saved to user.jsonl
```

To modify an existing prompt, use `/update_prompt prompt_name="better_commits"`.

### Method 2: Manual Editing

Edit `user.jsonl` directly:

```bash
cd ~/.local/hexai/data/prompts/
nano user.jsonl
```

Add a new prompt (one line, formatted for readability here):

```json
{"name":"optimize_sql","title":"Optimize SQL Query","description":"Analyzes and optimizes SQL queries for performance","arguments":[{"name":"query","description":"SQL query to optimize","required":true}],"messages":[{"role":"user","content":{"type":"text","text":"Optimize this SQL query:\n\n{{query}}\n\nSuggest improvements for:\n- Index usage\n- Query structure\n- Performance"}}],"tags":["database","optimization","sql"],"created":"2026-02-10T12:00:00Z","updated":"2026-02-10T12:00:00Z"}
```

**Tip**: Use a JSON formatter to validate:
```bash
cat user.jsonl | jq .
```

### Method 3: Python Script

Create prompts programmatically:

```python
import json
from datetime import datetime

prompt = {
    "name": "api_design",
    "title": "Design REST API",
    "description": "Designs a RESTful API endpoint with best practices",
    "arguments": [
        {
            "name": "resource",
            "description": "Resource name (e.g., 'users', 'posts')",
            "required": True
        },
        {
            "name": "operations",
            "description": "Operations to support (e.g., 'CRUD')",
            "required": False
        }
    ],
    "messages": [
        {
            "role": "user",
            "content": {
                "type": "text",
                "text": "Design a RESTful API for {{resource}}.\n\nOperations: {{operations}}\n\nInclude:\n- Endpoint paths\n- HTTP methods\n- Request/response formats\n- Status codes\n- Best practices"
            }
        }
    ],
    "tags": ["api", "design", "rest", "web"],
    "created": datetime.now().isoformat(),
    "updated": datetime.now().isoformat()
}

# Append to user.jsonl
with open("~/.local/hexai/data/prompts/user.jsonl", "a") as f:
    f.write(json.dumps(prompt) + "\n")
```

### Method 4: Go Code

Use hexai's promptstore package:

```go
package main

import (
    "time"
    "codeberg.org/snonux/hexai/internal/promptstore"
)

func main() {
    store, _ := promptstore.NewJSONLStore("~/.local/hexai/data/prompts/")

    prompt := &promptstore.Prompt{
        Name:        "security_audit",
        Title:       "Security Audit",
        Description: "Audits code for security vulnerabilities",
        Arguments: []promptstore.PromptArgument{
            {Name: "code", Description: "Code to audit", Required: true},
        },
        Messages: []promptstore.PromptMessage{
            {
                Role: "user",
                Content: promptstore.MessageContent{
                    Type: "text",
                    Text: "Audit this code for security issues:\n\n{{code}}",
                },
            },
        },
        Tags:    []string{"security", "audit", "vulnerability"},
        Created: time.Now(),
        Updated: time.Now(),
    }

    store.Create(prompt)
}
```

## Template Syntax

### Basic Substitution

Use `{{argument_name}}` to insert argument values:

```json
{
  "text": "Hello {{name}}! Your age is {{age}}."
}
```

When rendered with `{"name": "Alice", "age": "30"}`:
```
Hello Alice! Your age is 30.
```

### Multi-line Templates

Include newlines for formatting:

```json
{
  "text": "Code Review Report\n\n## Code\n{{code}}\n\n## Analysis\nReviewing for quality..."
}
```

### Optional Arguments

Use default text for optional arguments in your description:

```json
{
  "text": "Language: {{language}}\n\n(If not specified, will auto-detect)"
}
```

## Best Practices

### Naming Conventions

- **name**: Use lowercase with underscores: `code_review`, `generate_tests`
- **title**: Use Title Case: "Code Review", "Generate Tests"
- **argument names**: Use lowercase: `code`, `language`, `function_name`

### Description Guidelines

- Keep descriptions concise (1-2 sentences)
- Focus on what the prompt does, not how
- Mention key capabilities or outputs

Example:
```
✓ "Analyzes code for bugs and suggests fixes with explanations"
✗ "This prompt will take your code and use AI to find bugs"
```

### Argument Guidelines

- Mark arguments as `required: true` only if prompt can't work without them
- Use descriptive names: `code` not `c`, `error_message` not `err`
- Provide helpful descriptions for each argument

### Message Design

- Use clear, specific instructions
- Include examples when helpful
- Break complex requests into sections
- Use formatting (headers, lists) for readability

Example:
```json
{
  "text": "Review this code:\n\n{{code}}\n\nFocus on:\n- Performance issues\n- Security vulnerabilities\n- Code style violations\n- Best practices"
}
```

### Tags Strategy

Use tags to categorize prompts:

- **By language**: `go`, `python`, `javascript`, `rust`
- **By domain**: `web`, `cli`, `api`, `database`
- **By task**: `review`, `testing`, `documentation`, `refactoring`, `debugging`
- **By difficulty**: `beginner`, `intermediate`, `advanced`

Example tags:
```json
["go", "testing", "tdd", "unit-tests"]
```

## Example Prompts

### Code Optimization

```json
{
  "name": "optimize_algorithm",
  "title": "Optimize Algorithm",
  "description": "Analyzes algorithm complexity and suggests optimizations",
  "arguments": [
    {"name": "code", "description": "Algorithm implementation", "required": true},
    {"name": "constraints", "description": "Performance constraints", "required": false}
  ],
  "messages": [
    {
      "role": "user",
      "content": {
        "type": "text",
        "text": "Optimize this algorithm:\n\n{{code}}\n\nConstraints: {{constraints}}\n\nProvide:\n1. Time complexity analysis\n2. Space complexity analysis\n3. Optimization suggestions\n4. Optimized implementation"
      }
    }
  ],
  "tags": ["optimization", "performance", "algorithms"],
  "created": "2026-02-10T12:00:00Z",
  "updated": "2026-02-10T12:00:00Z"
}
```

### API Endpoint Design

```json
{
  "name": "design_endpoint",
  "title": "Design API Endpoint",
  "description": "Creates RESTful API endpoint specification",
  "arguments": [
    {"name": "resource", "description": "Resource name", "required": true},
    {"name": "operations", "description": "CRUD operations needed", "required": true}
  ],
  "messages": [
    {
      "role": "user",
      "content": {
        "type": "text",
        "text": "Design REST API endpoints for: {{resource}}\n\nOperations: {{operations}}\n\nSpecify:\n- Endpoint paths\n- HTTP methods\n- Request bodies\n- Response formats\n- Status codes\n- Authentication"
      }
    }
  ],
  "tags": ["api", "rest", "design", "web"],
  "created": "2026-02-10T12:00:00Z",
  "updated": "2026-02-10T12:00:00Z"
}
```

## Sharing Prompts

### Version Control

Store prompts in git for team collaboration:

```bash
cd ~/.local/hexai/data/prompts/
git init
git add user.jsonl
git commit -m "Add custom prompts"
git remote add origin https://github.com/myteam/hexai-prompts
git push -u origin main
```

### Team Setup

Team members clone the repository:

```bash
git clone https://github.com/myteam/hexai-prompts ~/hexai-prompts
export HEXAI_MCP_PROMPTS_DIR=~/hexai-prompts
```

### Updating Shared Prompts

```bash
cd ~/hexai-prompts
# Edit user.jsonl
git commit -am "Add SQL optimization prompt"
git push
```

## Troubleshooting

### Invalid JSON Format

**Error**: Prompt not appearing or parse warnings in logs

**Solution**: Validate JSON:
```bash
cat user.jsonl | jq . >/dev/null
# If errors, fix JSON syntax
```

### Duplicate Name

**Error**: "prompt already exists"

**Solution**: Prompts must have unique names. Change the name or delete the old prompt.

### Template Not Rendering

**Issue**: `{{argument}}` appears literally in output

**Cause**: Argument name mismatch

**Solution**: Ensure argument names match exactly (case-sensitive):
```json
"arguments": [{"name": "code", ...}]
"text": "{{code}}"  // ✓ Correct
"text": "{{Code}}"  // ✗ Won't work
```

### Missing Required Argument

**Error**: "missing required argument: X"

**Solution**: Provide all required arguments when using the prompt in your client.

## Advanced Topics

### Multi-Turn Conversations

Include both user and assistant messages for context:

```json
{
  "messages": [
    {
      "role": "user",
      "content": {"type": "text", "text": "What is {{topic}}?"}
    },
    {
      "role": "assistant",
      "content": {"type": "text", "text": "Let me explain {{topic}} in detail..."}
    },
    {
      "role": "user",
      "content": {"type": "text", "text": "Now apply this to: {{code}}"}
    }
  ]
}
```

### Conditional Logic

Use description to guide usage:

```json
{
  "description": "If 'language' is not provided, will auto-detect",
  "text": "Language: {{language}}\n\n(Auto-detect if empty)"
}
```

The MCP protocol doesn't support conditional logic in templates, but you can document expected behavior.

## See Also

- [MCP Setup Guide](mcp-setup.md)
- [hexai Configuration](configuration.md)
- [MCP Protocol Specification](https://modelcontextprotocol.io/)
