package perplexity

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/everstacklabs/sentinel/internal/adapter"
	"github.com/everstacklabs/sentinel/internal/httpclient"
)

func init() {
	adapter.Register(&Perplexity{})
}

// Perplexity adapter discovers models from Perplexity's documentation.
// Perplexity does not expose a public /models API endpoint.
type Perplexity struct {
	client *httpclient.Client
}

func (p *Perplexity) Name() string { return "perplexity" }

func (p *Perplexity) SupportedSources() []adapter.SourceType {
	return []adapter.SourceType{adapter.SourceDocs}
}

// Configure sets up the adapter with an HTTP client. No API key needed.
func (p *Perplexity) Configure(client *httpclient.Client) {
	p.client = client
}

func (p *Perplexity) Discover(ctx context.Context, opts adapter.DiscoverOptions) ([]adapter.DiscoveredModel, error) {
	var models []adapter.DiscoveredModel

	for _, src := range opts.Sources {
		switch src {
		case adapter.SourceDocs:
			docModels, err := p.discoverFromDocs(ctx)
			if err != nil {
				return nil, fmt.Errorf("perplexity docs discovery: %w", err)
			}
			models = append(models, docModels...)
		case adapter.SourceAPI:
			slog.Debug("perplexity has no models API, skipping API source")
		}
	}

	return models, nil
}
