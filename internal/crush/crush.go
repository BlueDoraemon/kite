// Package crush reads Crush's persisted provider configuration so Kite can
// reuse the selected large model, credential, and cached endpoint without
// executing crushrc. It never executes shell scripts.
package crush

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Imported is the resolved Crush configuration for Kite.
type Imported struct {
	// Provider is the provider type: "openai" or "openai-compat".
	Provider string
	// Model is the selected large model identifier.
	Model string
	// APIKey is the credential for the provider.
	APIKey string
	// Endpoint is the API endpoint for the provider.
	Endpoint string
}

// dataDir returns Crush's data directory.
func dataDir() string {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return filepath.Join(v, "crush")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "crush")
}

// Load reads Crush's persisted state and resolves the imported configuration.
func Load() (*Imported, error) {
	dir := dataDir()
	if dir == "" {
		return nil, fmt.Errorf("crush: cannot determine data directory")
	}

	// Read providers.json for provider definitions.
	prov, err := readProviders(filepath.Join(dir, "providers.json"))
	if err != nil {
		return nil, err
	}

	// Read crush.json for the selected model and credentials.
	sel, err := readSelection(filepath.Join(dir, "crush.json"))
	if err != nil {
		return nil, err
	}

	imp := &Imported{Model: sel.Models.Large.Model}
	if imp.Model == "" {
		return nil, fmt.Errorf("crush: no large model selected")
	}

	// Resolve the provider.
	providerID := sel.Models.Large.Provider
	p := prov.byID(providerID)
	if p == nil {
		return nil, fmt.Errorf("crush: provider %q is not configured", providerID)
	}
	switch p.Type {
	case "openai", "openai-compat":
		imp.Provider = p.Type
	case "hyper":
		return nil, fmt.Errorf("crush: the Hyper provider is not supported by kite; use an OpenAI or OpenAI-compatible provider")
	default:
		return nil, fmt.Errorf("crush: provider type %q is not supported by kite", p.Type)
	}

	imp.Endpoint = p.Endpoint
	if imp.Endpoint == "" {
		return nil, fmt.Errorf("crush: provider %q has no API endpoint", providerID)
	}

	// Resolve the credential. The API key may be an env-var reference.
	cred := sel.Providers[providerID]
	if isOAuthExpired(cred.OAuth) {
		return nil, fmt.Errorf("crush: the OAuth credential has expired or is near expiry; refresh it in Crush and try again")
	}
	key := cred.APIKey
	if key == "" {
		key = p.APIKey
	}
	if len(key) > 1 && key[0] == '$' {
		key = os.Getenv(key[1:])
	}
	if key == "" {
		return nil, fmt.Errorf("crush: provider %q has no credential", providerID)
	}
	imp.APIKey = key

	return imp, nil
}

// providerDef is a provider definition from providers.json.
type providerDef struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	APIKey   string `json:"api_key"`
	Endpoint string `json:"api_endpoint"`
}

// providers is the providers.json shape.
type providers struct {
	Items []providerDef `json:"items"`
}

func (p *providers) byID(id string) *providerDef {
	for i := range p.Items {
		if p.Items[i].ID == id {
			return &p.Items[i]
		}
	}
	return nil
}

func readProviders(path string) (*providers, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("crush: read providers: %w", err)
	}
	// providers.json may be a bare array.
	var arr []providerDef
	if err := json.Unmarshal(data, &arr); err == nil {
		return &providers{Items: arr}, nil
	}
	var p providers
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("crush: parse providers: %w", err)
	}
	return &p, nil
}

// selection is the crush.json shape relevant to Kite.
type selection struct {
	Providers map[string]providerCredential `json:"providers"`
	Models    struct {
		Large *modelSelection `json:"large"`
	} `json:"models"`
}

type providerCredential struct {
	APIKey string `json:"api_key"`
	OAuth  *oauth `json:"oauth,omitempty"`
}

type oauth struct {
	ExpiresAt int64 `json:"expires_at"`
}

type modelSelection struct {
	Model    string `json:"model"`
	Provider string `json:"provider"`
}

func readSelection(path string) (*selection, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("crush: read selection: %w", err)
	}
	var s selection
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("crush: parse selection: %w", err)
	}
	return &s, nil
}

// isOAuthExpired reports whether the OAuth credential is expired or within
// 5 minutes of expiry.
func isOAuthExpired(o *oauth) bool {
	if o == nil || o.ExpiresAt == 0 {
		return false
	}
	return time.Now().Unix()+300 >= o.ExpiresAt
}
