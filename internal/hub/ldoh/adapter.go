package ldoh

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/lingyuins/octopus/internal/model"
)

// DiscoveredSite represents a site found via public directory.
type DiscoveredSite struct {
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
	Type    string `json:"type"`
	Status  string `json:"status"`
}

// DiscoverSites queries known public directories and returns discovered sites.
func DiscoverSites(ctx context.Context) ([]DiscoveredSite, error) {
	directories := []string{
		"https://api.ldoh.cloud/sites",
	}

	var allSites []DiscoveredSite

	client := &http.Client{Timeout: 15 * time.Second}
	for _, dirURL := range directories {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, dirURL, nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		var sites []DiscoveredSite
		if err := json.Unmarshal(body, &sites); err != nil {
			continue
		}

		for i := range sites {
			if sites[i].Type == "" {
				sites[i].Type = model.SiteTypeUnknown
			}
			sites[i].BaseURL = strings.TrimRight(sites[i].BaseURL, "/")
		}
		allSites = append(allSites, sites...)
	}

	if len(allSites) == 0 {
		return nil, fmt.Errorf("no sites discovered")
	}
	return allSites, nil
}
