# Provider Adapter Tracker

Status of adapter implementations for each provider.

## Legend
- [x] Done
- [ ] Todo

## Tier 1 — Major API Providers
These have first-party model APIs with `/models` or equivalent endpoints.

- [x] **OpenAI** — GPT, O-series, embeddings (`/v1/models`)
- [x] **Anthropic** — Claude opus/sonnet/haiku (`/v1/models`)
- [ ] **Google (Gemini)** — Gemini Pro/Flash/Ultra (`/v1beta/models`)
- [ ] **Mistral AI** — Mistral Large/Medium/Small, Mixtral, Codestral (`/v1/models`)
- [ ] **Cohere** — Command R/R+, Embed, Rerank (`/v1/models`)
- [ ] **xAI** — Grok models (`/v1/models`, OpenAI-compatible)
- [ ] **DeepSeek** — DeepSeek V3/R1, Coder (`/models`, OpenAI-compatible)
- [ ] **Meta (Llama API)** — Llama 4 Scout/Maverick (`/v1/models`)

## Tier 2 — Inference Platforms
Host multiple providers' models behind a unified API.

- [ ] **Together AI** — Open-source model hosting (`/v1/models`, OpenAI-compatible)
- [ ] **Fireworks AI** — Fast inference platform (`/v1/models`, OpenAI-compatible)
- [ ] **Groq** — LPU-accelerated inference (`/openai/v1/models`, OpenAI-compatible)
- [ ] **Perplexity** — Search-augmented models (`/chat/completions`, OpenAI-compatible)
- [ ] **Azure OpenAI** — Microsoft-hosted OpenAI models (custom endpoint pattern)
- [ ] **OpenRouter** — Multi-provider gateway (`/api/v1/models`)
- [ ] **AWS Bedrock** — Amazon-hosted models (AWS SDK, not REST)
- [ ] **Replicate** — Model hosting platform (`/v1/models`)

## Tier 3 — Specialized / Regional
- [ ] **AI21 Labs** — Jamba models (`/v1/models`)
- [ ] **Alibaba (Qwen/DashScope)** — Qwen models
- [ ] **Nvidia NIM** — Nvidia-hosted models
- [ ] **Databricks (DBRX)** — Databricks-hosted models
- [ ] **Reka** — Reka Core/Flash/Edge
- [ ] **Writer** — Palmyra models
- [ ] **Inflection** — Pi models
- [ ] **Zhipu AI (GLM)** — ChatGLM models
- [ ] **Minimax** — abab models
- [ ] **Moonshot AI (Kimi)** — Kimi/Moonshot models
- [ ] **ByteDance (Doubao)** — Doubao models
- [ ] **Baichuan** — Baichuan models
- [ ] **01.AI (Yi)** — Yi models
- [ ] **Stability AI** — Stable Diffusion/LM models

## Notes

- **OpenAI-compatible** providers (xAI, DeepSeek, Together, Fireworks, Groq) can potentially share a generic OpenAI-compatible adapter with per-provider config for skip rules, families, and capabilities.
- Tier 1 providers should be prioritized — they have the most unique API shapes.
- Tier 2 inference platforms often use OpenAI-compatible APIs, making them lower effort.
