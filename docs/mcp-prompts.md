# MCP Prompts Guide

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

hexai-mcp-server includes these built-in prompts:

### code_review
Analyzes code quality, style, and potential issues.

**Arguments**:
- `code` (required): The code to review

**Tags**: development, review, quality

### explain_code
Provides detailed explanation of what code does.

**Arguments**:
- `code` (required): The code to explain

**Tags**: development, documentation, learning

### generate_tests
Generates unit tests for a function or class.

**Arguments**:
- `code` (required): The code to test
- `language` (optional): Programming language

**Tags**: development, testing, tdd

### document_function
Generates documentation comments and docstrings.

**Arguments**:
- `code` (required): The code to document

**Tags**: development, documentation

### simplify_code
Simplifies complex code while preserving behavior.

**Arguments**:
- `code` (required): The code to simplify

**Tags**: development, refactoring, quality

### fix_bugs
Analyzes code for bugs and suggests fixes.

**Arguments**:
- `code` (required): The code to analyze
- `error` (optional): Error message or symptoms

**Tags**: development, debugging, bug-fix

### refactor_extract
Extracts code into a separate, reusable function.

**Arguments**:
- `code` (required): The code to extract
- `function_name` (optional): Desired function name

**Tags**: development, refactoring

## Creating Custom Prompts

### Storage Files

Prompts are stored in two files:
- `default.jsonl`: Built-in prompts (automatically created)
- `user.jsonl`: Your custom prompts

Both files are in: `~/.local/share/hexai/prompts/` (or your configured directory)

### Method 1: Manual Editing

Edit `user.jsonl` directly:

```bash
cd ~/.local/share/hexai/prompts/
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

### Method 2: Python Script

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
with open("~/.local/share/hexai/prompts/user.jsonl", "a") as f:
    f.write(json.dumps(prompt) + "\n")
```

### Method 3: Go Code

Use hexai's promptstore package:

```go
package main

import (
    "time"
    "codeberg.org/snonux/hexai/internal/promptstore"
)

func main() {
    store, _ := promptstore.NewJSONLStore("~/.local/share/hexai/prompts/")

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
cd ~/.local/share/hexai/prompts/
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
