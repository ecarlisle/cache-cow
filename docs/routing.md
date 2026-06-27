# Routing Heuristics

The router classifies each request as `simple` or `complex`. Simple requests go to the cheap model (`cheap_model`), complex to the expensive model (`expensive_model`).

## Classification Logic

A request is `complex` if any of these conditions are true:

### 1. Tool Calls

- The request includes a `tools` array (function/tool definitions)
- Any message has `role: "tool"` (a tool result is being returned)

Toggle: `route_tool_calls_to_expensive` (default `true`).

### 2. Long Messages

- Any single message body exceeds `long_context_threshold` bytes (default 3000)

Toggle: `route_long_contexts_to_expensive` (default `true`).

### 3. Many Messages

- More than 6 non-system messages in the conversation

This is a proxy for multi-turn conversations that likely involve complex reasoning.

### 4. Code Generation Indicators

- A user message exceeds 100 bytes AND contains code patterns:
  - Code fences (` ``` `)
  - Language keywords (`function`, `class `, `impl `, `def `)
- Or action verbs that indicate code generation:
  - `write a`, `create a`, `implement`, `build a`, `refactor`

## When to Add a Heuristic

New heuristics should be:

1. **Toggleable** — added to `RouterConfig` so they can be disabled independently
2. **Documented** — added to this file with the config key and default value
3. **Tested** — table-driven unit test in `router_test.go` covering both sides of the decision boundary
