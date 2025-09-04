package testutil

// MultilineDocBlock returns a realistic multi-line documentation block.
func MultilineDocBlock() string {
    return "// add adds two numbers\n// returns their sum"
}

// MultilineChatReply returns a multi-line assistant reply for chat tests.
func MultilineChatReply() string {
    return "Hello, world!\nThis is a multi-line reply."
}

// MultilineFunctionSuggestion returns a more realistic multi-line function body suggestion.
func MultilineFunctionSuggestion() string {
    return "(ctx context.Context, input string) (*CustData, error) {\n    // TODO: implement\n    return &CustData{}, nil\n}"
}

// MarkdownCodeFence returns a fenced markdown snippet used in post-processing tests.
func MarkdownCodeFence() string {
    return "```go\nname := value\n```"
}

// MalformedJSON returns a deliberately malformed JSON string.
func MalformedJSON() string {
    return "{\"choices\":[{\"delta\":{\"content\":\"oops\"}}]"
}

