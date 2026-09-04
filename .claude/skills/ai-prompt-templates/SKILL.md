---
name: ai-prompt-templates
description: Work with the AI layer in this D&D campaign manager — add or edit prompt templates, add a Service method, tune temperature, handle streaming, retries, rate limits, token cost, or switch AI provider (Groq, DeepSeek, OpenAI, local). Use this skill whenever touching internal/infrastructure/ai/, changing what the AI DM says, adding narrative/NPC dialogue/quest/backstory generation, wiring AI into an endpoint, changing the model or provider, or debugging a prompt that came out with literal {{placeholders}} in it or a 429 — even if the user just says "make the DM describe the scene better". Variable substitution here is plain string replacement with no escaping and no conditionals, and the provider is config-only, both of which surprise people.
---

# AI prompt templates and service

`internal/infrastructure/ai/` — four files:

| File | Role |
|---|---|
| `client.go` | `Client` interface, request/response types, `Error`, `Pricing` |
| `openai_compatible.go` | `OpenAICompatibleClient`, the only implementation |
| `prompts.go` | `PromptBuilder`, `defaultPrompts()`, `TemperatureSettings` |
| `service.go` | `Service` — the high-level API the rest of the app should call |

Nothing outside this package should construct a client. Depend on `ai.Service`.

## The provider is configuration, never code

`OpenAICompatibleClient` speaks the OpenAI `/chat/completions` shape, which Groq (the
default), DeepSeek, OpenAI and local servers all serve. The vendor is decided by
`ai.base_url` and `ai.model` in `configs/config.yaml`.

**No Go symbol may name a provider.** If a change would put "groq", "deepseek" or "openai"
into a Go identifier, string literal or default, it belongs in config instead. Swapping
providers has to stay a config edit.

- `ai.api_key` binds to `GROQ_API_KEY`, `DEEPSEEK_API_KEY`, `OPENAI_API_KEY` and
  `AI_API_KEY`, in that order — first one set wins.
- There is **no default** `base_url` or `model`. `ClientConfig.Validate` fails at startup
  rather than guessing an endpoint. Do not add defaults for these.
- Pricing lives in `ai.pricing` (USD per million tokens) because rates differ per provider
  and model. Both zero disables cost estimation.
- **Model IDs churn.** Groq retires and renames models regularly. Never hardcode one; when
  a model 404s, check the provider's `/models` endpoint rather than trusting any written
  note, this file included.

**The AI layer is not wired into the HTTP API yet.** `examples/ai_usage.go` is the only
caller. Connecting it means adding a handler (see the `add-api-endpoint` skill) and
constructing `ai.NewService` in `main.go` from `cfg.DeepSeek`.

## Substitution is single-pass string replacement, not a template engine

`BuildPrompt` scans each template once with `placeholderPattern` and replaces every
`{{key}}` from the variables map. Consequences:

- **A missing variable is an error**, listing the names it could not resolve. It used to
  leave the braces in place and ship `{{party_status}}` to the model. Pass every variable
  the template names.
- **Substituted values are inert.** A `{{other_key}}` appearing inside player text is not
  expanded. Replacing key by key did expand it, and Go randomises map order, so whether a
  player could inject another variable varied run to run.
- **Conditionals still do not work.** Never write `{{#if x}}`; nothing evaluates it and a
  test fails the build if one appears. Pre-format the optional line in Go and pass it as a
  plain variable — `combat_narration` takes `damage_line` for exactly this.
- **Values are not escaped.** `player_input` and `player_message` are player-controlled
  and go straight into the user prompt. Keep untrusted text out of the *system* prompt,
  which is where the DM's rules live.

## Adding a template

1. Add the entry to `defaultPrompts()` in `prompts.go` as a `{System, User}` pair.
   System carries role, personality and output format; User carries the turn's content
   and context. Keep the existing markdown-with-headings house style.
2. Add a temperature to `TemperatureSettings` keyed by task type. The convention:
   0.3 mechanical/consistent, 0.5–0.6 balanced, 0.7–0.8 creative.
3. Add a `Service` method (below). A template with no method is unreachable — five of the
   eight are currently orphaned that way (`narrative_generation`, `combat_narration`,
   `story_adaptation`, `character_backstory`, `quest_generation`).
4. Every `{{placeholder}}` must have a value at call time, or `BuildPrompt` errors.

`AddTemplate` exists for runtime registration but nothing uses it; prefer
`defaultPrompts()` so the template is visible in source.

## Adding a Service method

Every method follows the same eight steps — copy `GenerateNarrative`:

```go
func (s *Service) DoThing(ctx context.Context, req *ThingRequest) (*ThingResponse, error) {
    startTime := time.Now()

    variables := map[string]string{ /* every placeholder the template names */ }

    messages, err := s.promptBuilder.BuildPrompt("template_name", variables)
    if err != nil {
        return nil, fmt.Errorf("failed to build prompt: %w", err)
    }

    chatReq := &ChatRequest{
        Messages:    messages,
        Model:       s.config.Model,
        Temperature: GetTemperature("task_type"),
        MaxTokens:   500,
        TopP:        0.9,
    }

    resp, err := s.client.ChatCompletion(ctx, chatReq)
    if err != nil {
        return nil, fmt.Errorf("AI request failed: %w", err)
    }
    if len(resp.Choices) == 0 {
        return nil, fmt.Errorf("no response from AI")
    }

    return &ThingResponse{
        Content:        resp.Choices[0].Message.Content,
        TokensUsed:     resp.Usage.TotalTokens,
        Cost:           calculateCost(resp.Usage),
        ProcessingTime: time.Since(startTime),
    }, nil
}
```

`MaxTokens` by shape of output: ~300 dice interpretation, ~500 dialogue, ~1000 narrative.
Every response struct carries `TokensUsed`, `Cost` and `ProcessingTime` — keep that, the
Session model aggregates them via `UpdateAIInteractions`.

Use `BuildConversation` instead of `BuildPrompt` when there is history; it splices
history between the system and user messages. Only `GenerateNPCDialogue` does this today.

## Retries, streaming, cost

- Retry lives in `ChatCompletion` and in `openStream`, not the transport: up to
  `MaxRetries` (default 3), driven by `Error.Retriable`, true for 429/500/502/503/504 and
  network errors. A non-retriable `*Error` returns immediately.
- `waitBefore` picks the delay: a provider's own `Retry-After` header wins when present,
  otherwise `backoffFor(attempt)` (1s, 2s, 4s…). Both are capped at `maxRetryWait` (30s)
  so a provider naming an absurd delay cannot block a player's turn. Honouring the header
  matters on a free tier — guessing spends quota discovering what the header already said.
- **`StreamChatCompletion` returns a `*ChatStream`.** Range over `Chunks`, then check
  `Err()`. Opening the stream is retried; once bytes are flowing a failure cannot be
  retried without replaying the partial completion, so it surfaces through `Err()` —
  a truncated stream otherwise closes its channel exactly like a clean one.
- The SSE reader skips blank lines and `:` comments, tolerates lines up to 1MB
  (`bufio.Scanner` defaults to 64KB and would have failed silently), and drops
  unparseable JSON chunks.
- `StreamNarrative` returns `(<-chan string, <-chan error)`; both close when the goroutine
  returns. Range over the text channel, then check the error channel — it now carries
  mid-stream failures too.
- `Service.calculateCost(usage)` applies the configured `Pricing`, charging prompt and
  completion tokens at their separate rates. An unconfigured provider reports 0 rather
  than a wrong number. Always an estimate — never use it for billing.

## Configuration

`ai.ClientConfig` comes from `cfg.AI`. `NewService` calls `Validate` first, so a missing
key, base URL or model fails at construction rather than as a 401 mid-turn. Timeout
(30s) and retries (3) default; routing does not.

The HTTP client timeout is per-request and covers the *whole* streaming response, so a
long narration can be cut off by a short `ai.timeout`. Groq answers in well under a
second, which is why the default config sets 10s — long enough for a full stream, short
enough to surface a real problem quickly.

## Checklist

- [ ] Every `{{placeholder}}` in the template has a key in the variables map
      (`BuildPrompt` errors otherwise)
- [ ] No Handlebars-style conditionals — build the string in Go instead
- [ ] Untrusted player text stays out of the system prompt
- [ ] Temperature key added to `TemperatureSettings`
- [ ] `Service` method exists, else the template is unreachable
- [ ] Response carries tokens, cost and processing time
- [ ] `ctx` threaded through, never `context.Background()`
- [ ] Streaming callers check `ChatStream.Err()` after draining `Chunks`
- [ ] No provider name introduced into Go code — routing stays in config
- [ ] `go test ./internal/infrastructure/ai/...` passes
