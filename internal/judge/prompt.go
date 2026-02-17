package judge

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/everstacklabs/sentinel/internal/diff"
)

func buildSystemPrompt() string {
	return `You are a model catalog reviewer for an AI gateway. Your job is to evaluate proposed changes to a model catalog and identify issues.

For each model in the changeset, evaluate:

1. **Capabilities**: Are the inferred capabilities reasonable for this model type? (e.g., an embedding model should NOT have "chat" or "function_calling")
2. **Pricing**: Is the pricing plausible? Compare against known market rates. Flag suspiciously high or low prices.
3. **Limits**: Are the token limits reasonable? (e.g., max_completion_tokens should not exceed max_tokens, context windows should match known specs)
4. **Status**: Is the status appropriate? (e.g., a brand-new model shouldn't be "deprecated")
5. **Changes**: For updated models, are the field changes plausible? (e.g., a price dropping 90% is suspicious)

Respond with a JSON object containing a "verdicts" array. Each verdict must have:
- "model_name": the model identifier
- "verdict": one of "approve", "flag", or "reject"
  - "approve": the model data looks correct
  - "flag": something looks suspicious but might be correct — needs human review
  - "reject": the data is clearly wrong and should not be merged
- "confidence": a float between 0 and 1 indicating your confidence
- "concerns": an array of strings describing specific issues (empty if approved)
- "reasoning": a brief explanation of your assessment

Be conservative: prefer "flag" over "reject" unless the data is clearly incorrect.
Only "reject" when you are highly confident the data is wrong (e.g., an embedding model with chat capabilities, negative pricing, max_completion_tokens > max_tokens).

Respond ONLY with the JSON object, no other text.`
}

func buildUserPrompt(cs *diff.ChangeSet) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Provider: %s\n\n", cs.Provider))

	if len(cs.New) > 0 {
		b.WriteString("## New Models\n\n")
		for _, m := range cs.New {
			data := modelSummary{
				Name:         m.Name,
				Family:       m.Model.Family,
				Status:       m.Model.Status,
				Capabilities: m.Model.Capabilities,
				Modalities: modalitySummary{
					Input:  m.Model.Modalities.Input,
					Output: m.Model.Modalities.Output,
				},
				Limits: limitsSummary{
					MaxTokens:           m.Model.Limits.MaxTokens,
					MaxCompletionTokens: m.Model.Limits.MaxCompletionTokens,
				},
			}
			if m.Model.Cost != nil {
				data.Cost = &costSummary{
					InputPer1K:  m.Model.Cost.InputPer1K,
					OutputPer1K: m.Model.Cost.OutputPer1K,
				}
			}
			jsonBytes, _ := json.MarshalIndent(data, "", "  ")
			b.WriteString(fmt.Sprintf("```json\n%s\n```\n\n", string(jsonBytes)))
		}
	}

	if len(cs.Updated) > 0 {
		b.WriteString("## Updated Models\n\n")
		for _, u := range cs.Updated {
			data := updateSummary{
				Name: u.Name,
			}
			for _, c := range u.Changes {
				data.Changes = append(data.Changes, changeSummary{
					Field:    c.Field,
					OldValue: c.OldValue,
					NewValue: c.NewValue,
				})
			}
			// Include full model state for context
			data.CurrentState = modelSummary{
				Name:         u.Name,
				Family:       u.Model.Family,
				Status:       u.Model.Status,
				Capabilities: u.Model.Capabilities,
				Modalities: modalitySummary{
					Input:  u.Model.Modalities.Input,
					Output: u.Model.Modalities.Output,
				},
				Limits: limitsSummary{
					MaxTokens:           u.Model.Limits.MaxTokens,
					MaxCompletionTokens: u.Model.Limits.MaxCompletionTokens,
				},
			}
			if u.Model.Cost != nil {
				data.CurrentState.Cost = &costSummary{
					InputPer1K:  u.Model.Cost.InputPer1K,
					OutputPer1K: u.Model.Cost.OutputPer1K,
				}
			}
			jsonBytes, _ := json.MarshalIndent(data, "", "  ")
			b.WriteString(fmt.Sprintf("```json\n%s\n```\n\n", string(jsonBytes)))
		}
	}

	return b.String()
}

type modelSummary struct {
	Name         string          `json:"name"`
	Family       string          `json:"family"`
	Status       string          `json:"status"`
	Capabilities []string        `json:"capabilities"`
	Modalities   modalitySummary `json:"modalities"`
	Limits       limitsSummary   `json:"limits"`
	Cost         *costSummary    `json:"cost,omitempty"`
}

type modalitySummary struct {
	Input  []string `json:"input"`
	Output []string `json:"output"`
}

type limitsSummary struct {
	MaxTokens           int `json:"max_tokens"`
	MaxCompletionTokens int `json:"max_completion_tokens,omitempty"`
}

type costSummary struct {
	InputPer1K  float64 `json:"input_per_1k"`
	OutputPer1K float64 `json:"output_per_1k"`
}

type updateSummary struct {
	Name         string        `json:"name"`
	Changes      []changeSummary `json:"changes"`
	CurrentState modelSummary  `json:"current_state"`
}

type changeSummary struct {
	Field    string      `json:"field"`
	OldValue interface{} `json:"old_value"`
	NewValue interface{} `json:"new_value"`
}
