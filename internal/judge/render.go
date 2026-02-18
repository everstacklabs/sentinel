package judge

import (
	"fmt"
	"strings"
)

// RenderSection generates a markdown section for the PR body summarizing judge results.
// Returns an empty string when all models are approved or result is nil.
func RenderSection(result *Result) string {
	if result == nil {
		return ""
	}

	var flagged, rejected []ModelVerdict
	for _, v := range result.Verdicts {
		switch v.Verdict {
		case VerdictFlag:
			flagged = append(flagged, v)
		case VerdictReject:
			rejected = append(rejected, v)
		}
	}

	if len(flagged) == 0 && len(rejected) == 0 {
		return ""
	}

	var b strings.Builder

	b.WriteString("### LLM Judge Review\n\n")

	approved := len(result.Verdicts) - len(flagged) - len(rejected)
	fmt.Fprintf(&b, "**%d** approved, **%d** flagged, **%d** rejected\n\n",
		approved, len(flagged), len(rejected))

	if len(rejected) > 0 {
		b.WriteString("<details>\n<summary>Rejected Models</summary>\n\n")
		b.WriteString("| Model | Confidence | Concerns | Reasoning |\n")
		b.WriteString("|-------|-----------|----------|----------|\n")
		for _, v := range rejected {
			concerns := strings.Join(v.Concerns, "; ")
			fmt.Fprintf(&b, "| `%s` | %.0f%% | %s | %s |\n",
				v.ModelName, v.Confidence*100, concerns, v.Reasoning)
		}
		b.WriteString("\n</details>\n\n")
	}

	if len(flagged) > 0 {
		b.WriteString("<details>\n<summary>Flagged Models</summary>\n\n")
		b.WriteString("| Model | Confidence | Concerns | Reasoning |\n")
		b.WriteString("|-------|-----------|----------|----------|\n")
		for _, v := range flagged {
			concerns := strings.Join(v.Concerns, "; ")
			fmt.Fprintf(&b, "| `%s` | %.0f%% | %s | %s |\n",
				v.ModelName, v.Confidence*100, concerns, v.Reasoning)
		}
		b.WriteString("\n</details>\n\n")
	}

	return b.String()
}
