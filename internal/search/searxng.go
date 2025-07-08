package search

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/MehmetMHY/cha-go/pkg/types"
)

// SearXNGClient handles web search operations
type SearXNGClient struct {
	baseURL string
	client  *http.Client
}

// NewSearXNGClient creates a new SearXNG client
func NewSearXNGClient(baseURL string) *SearXNGClient {
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	
	return &SearXNGClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// Search performs a web search using SearXNG
func (s *SearXNGClient) Search(query string) ([]types.SearXNGResult, error) {
	apiURL := fmt.Sprintf("%s/search", s.baseURL)
	
	params := url.Values{}
	params.Add("q", query)
	params.Add("format", "json")
	
	fullURL := fmt.Sprintf("%s?%s", apiURL, params.Encode())
	
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("SearXNG is not running or not accessible at %s", s.baseURL)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == 403 {
		return nil, fmt.Errorf("SearXNG is blocking API requests. Please check your SearXNG configuration")
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SearXNG search failed with status: %d", resp.StatusCode)
	}
	
	var searchResponse types.SearXNGResponse
	err = json.NewDecoder(resp.Body).Decode(&searchResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SearXNG response: %v", err)
	}
	
	return searchResponse.Results, nil
}

// FormatResults formats search results into a context string for AI processing
func (s *SearXNGClient) FormatResults(results []types.SearXNGResult, query string) string {
	if len(results) == 0 {
		return fmt.Sprintf("I searched for '%s' but didn't find any results. Please try a different query.", query)
	}
	
	var context strings.Builder
	context.WriteString(fmt.Sprintf("I searched for '%s' and found the following results. Please provide a comprehensive answer based on these sources using IEEE citation format:\n\n", query))
	
	maxResults := 8
	if len(results) > maxResults {
		results = results[:maxResults]
	}
	
	for i, result := range results {
		context.WriteString(fmt.Sprintf("[%d] %s\n", i+1, result.Title))
		context.WriteString(fmt.Sprintf("URL: %s\n", result.URL))
		
		content := strings.TrimSpace(result.Content)
		if len(content) > 300 {
			content = content[:300] + "..."
		}
		context.WriteString(fmt.Sprintf("Content: %s\n\n", content))
	}
	
	context.WriteString("Please provide a comprehensive answer based on these search results. Use IEEE citation format with citations like [1], [2], etc., and include a References section at the end listing all sources with their URLs in the format:\n\nReferences:\n[1] Title, URL\n[2] Title, URL\netc.")
	
	return context.String()
}