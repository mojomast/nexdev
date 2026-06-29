package config

import (
	"fmt"

	keyring "github.com/zalando/go-keyring"
)

const keyringService = "geoffrussy"

func setSecret(provider, secret string) error {
	if provider == "" || secret == "" {
		return fmt.Errorf("invalid secret data")
	}
	return keyring.Set(keyringService, provider, secret)
}

func getSecret(provider string) (string, error) {
	if provider == "" {
		return "", fmt.Errorf("invalid provider")
	}
	return keyring.Get(keyringService, provider)
}
