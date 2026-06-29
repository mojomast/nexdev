package mcp

import (
	"fmt"

	"github.com/mojomast/nexdev/internal/config"
	"github.com/mojomast/nexdev/internal/provider"
)

func initProviderForStage(cfgMgr *config.Manager, stage, overrideModel string) (provider.Provider, string, error) {
	providerName, modelName, err := getProviderAndModel(cfgMgr, stage, overrideModel)
	if err != nil {
		return nil, "", err
	}

	p, err := provider.CreateProvider(providerName)
	if err != nil {
		return nil, "", err
	}

	if providerName == "ollama" {
		if err := p.Authenticate(""); err != nil {
			return nil, "", err
		}
	} else {
		apiKey, err := cfgMgr.GetAPIKey(providerName)
		if err != nil {
			return nil, "", err
		}
		if err := p.Authenticate(apiKey); err != nil {
			return nil, "", err
		}
	}

	if !p.IsAuthenticated() {
		return nil, "", fmt.Errorf("provider %s failed authentication", providerName)
	}

	return p, modelName, nil
}
