# Context Window Management

The context manager applies a sliding window to keep the conversation within `max_context_turns` non-system messages.

## When It Activates

Only when the number of non-system messages exceeds `max_context_turns` (default 20).

## How It Works

```
Original: [sys] [sys] [1] [2] [3] [4] [5] [6] [7] [8] [9] [10] [11] [12] [13]
                                                                   ↑ max_turns=10

After trim: [sys] [sys] [1] [2] [3] [4] [5]  [summary: 5 omitted]  [9] [10] [11] [12] [13]
                        └─ keep first N/2 ─┘                       └─ keep last N/2 ─┘
```

1. All system messages are preserved.
2. The first `max_context_turns / 2` non-system messages are kept.
3. The last `max_context_turns / 2` non-system messages are kept.
4. Everything in the middle is replaced with a single synthetic system message: `[summary of N previous messages omitted to conserve tokens]`.

## Limitations

- The summary is static (placeholder text, not LLM-generated). The middle messages are dropped entirely — no semantic compression.
- A future improvement would call a cheap model to produce a real summary of the truncated messages.

## Configuration

Set `max_context_turns` in config (default 20). Higher values preserve more context but increase token usage. Lower values activate sooner and save more tokens but risk losing information.
