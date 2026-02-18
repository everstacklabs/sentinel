package ai21

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/everstacklabs/sentinel/internal/adapter"
	"github.com/everstacklabs/sentinel/internal/httpclient"
)

func init() {
	adapter.Register(&AI21{})
}

// AI21 adapter discovers models from AI21 Labs' documentation.
// AI21 does not expose a public /models API endpoint.
type AI21 struct {
	client *httpclient.Client
}

func (a *AI21) Name() string { return "ai21" }

func (a *AI21) SupportedSources() []adapter.SourceType {
	return []adapter.SourceType{adapter.SourceDocs}
}

// Configure sets up the adapter with an HTTP client. No API key needed.
func (a *AI21) Configure(client *httpclient.Client) {
	a.client = client
}

func (a *AI21) Discover(ctx context.Context, opts adapter.DiscoverOptions) ([]adapter.DiscoveredModel, error) {
	var models []adapter.DiscoveredModel

	for _, src := range opts.Sources {
		switch src {
		case adapter.SourceDocs:
			docModels, err := a.discoverFromDocs(ctx)
			if err != nil {
				return nil, fmt.Errorf("ai21 docs discovery: %w", err)
			}
			models = append(models, docModels...)
		case adapter.SourceAPI:
			slog.Debug("ai21 has no models API, skipping API source")
		}
	}

	return models, nil
}
