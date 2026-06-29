# Providers and Models

## Supported Providers

- `openai`, `openai-codex`
- `anthropic`
- `zai`, `kimi`, `firmware`, `requesty`
- `openrouter`, `groq`, `together`, `deepinfra`, `fireworks`, `perplexity`, `mistral`
- `ollama`
- `opencode`

## Discovery Behavior

Geoffrussy tries to discover models from provider APIs when possible:

- Uses provider `DiscoverModels()` where implemented.
- Uses `ListModels()` and merges/deduplicates results.
- Shows usage signals (rate/quota) when available in provider APIs.

## Assigning Models Per Pipeline Step

Use:

```bash
geoffrussy config --set-model
```

Recommended granular keys:

- `interview.run`
- `interview.followup`
- `interview.analysis`
- `interview.defaults`
- `design.generate`
- `design.refine`
- `devplan.generate`
- `review.phase`
- `develop.execute`
- `develop.blocker_analyze`

Fallback resolution uses parent keys (for example `design.generate` falls back to `design`).

## Provider Onboarding

```bash
geoffrussy config --provider-help <provider>
```

This shows key format, notes, and docs URL.

## Copy-Paste Setup Examples

### OpenAI

```bash
geoffrussy config --provider-help openai
geoffrussy config --set-key
geoffrussy config --set-model
```

Suggested stage defaults:

- `interview.run`: `gpt-4o`
- `devplan.generate`: `gpt-4o`
- `develop.execute`: `gpt-4.1`

### Anthropic

```bash
geoffrussy config --provider-help anthropic
geoffrussy config --set-key
geoffrussy config --set-model
```

Suggested stage defaults:

- `design.generate`: `claude-3-5-sonnet`
- `review.phase`: `claude-3-5-sonnet`

### OpenRouter

```bash
geoffrussy config --provider-help openrouter
geoffrussy config --set-key
geoffrussy config --list-providers
```

OpenRouter is useful when you want one key with broad model access.

### Groq

```bash
geoffrussy config --provider-help groq
geoffrussy config --set-key
geoffrussy config --list-providers
```

Good for low-latency Llama/Mixtral class models.

### Together / DeepInfra / Fireworks

```bash
geoffrussy config --provider-help together
geoffrussy config --provider-help deepinfra
geoffrussy config --provider-help fireworks
geoffrussy config --set-key
```

These are useful for broad open-model coverage.

### Mistral / Perplexity

```bash
geoffrussy config --provider-help mistral
geoffrussy config --provider-help perplexity
geoffrussy config --set-key
```

### Z.ai (GLM) and Kimi

```bash
geoffrussy config --provider-help zai
geoffrussy config --provider-help kimi
geoffrussy config --set-key
```

### Ollama (Local)

```bash
ollama serve
geoffrussy config --provider-help ollama
geoffrussy config --list-providers
```

No API key required.

### OpenAI Codex Business

```bash
geoffrussy config --provider-help openai-codex
geoffrussy config --set-key
```

If your org uses SSO/business auth flows, complete provider-side web auth first and then store the usable credential.

## Verify Provider Availability

```bash
geoffrussy config --list-providers
geoffrussy quota --refresh
```

This validates authentication and fetches model/rate/quota data where APIs provide it.

## Secure Key Storage

- Primary: OS keyring
- Fallback: config file value with source metadata
- Environment variables override configured values

View current configuration and key source status:

```bash
geoffrussy config
```

## OpenAI Codex Business Note

`openai-codex` is supported as a provider identity and selection target. If your org requires web SSO/business flows, complete provider-side auth first and then configure the usable credential in Geoffrussy.
