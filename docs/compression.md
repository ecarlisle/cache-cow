# Compression

The compressor reduces token usage by normalizing whitespace and truncating oversized tool results.

## Whitespace Normalization

For each text-content message:

1. All non-newline whitespace characters (tabs, carriage returns, non-breaking spaces, etc.) are replaced with a plain space.
2. The message is split on newlines.
3. Each line is trimmed of leading/trailing whitespace.
4. Empty lines (blank after trim) are removed.
5. Lines are rejoined with newlines.

### Effect on Different Inputs

| Input | Output |
|-------|--------|
| `"hello\t\tworld\n\n\nhow are you?"` | `"hello  world\nhow are you?"` |
| `"  hello   world  "` | `"hello   world"` |
| `"a\n\n\nb"` | `"a\nb"` |
| `"   \n  \n  "` | (message removed entirely) |

### Guard

If compression would reduce the message to less than 1/3 of its original byte length, the original is kept. This prevents messages that are mostly content (not whitespace) from being needlessly re-serialized.

## Tool Message Truncation

Tool-role messages that exceed `compress_token_threshold` bytes (default 4000) are truncated to that length with `\n...[truncated]` appended. This prevents large tool results from consuming the context window.

Other message roles (system, user, assistant) are **not** truncated — only tool results are subject to length limits.

## Empty Message Removal

Messages whose content is empty after whitespace normalization are dropped entirely. This eliminates messages that are purely whitespace or empty strings.
