package scraper

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// ScrapedContent represents the result of scraping
type ScrapedContent struct {
	URL       string                 `json:"url"`
	Content   string                 `json:"content,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Timestamp int64                  `json:"timestamp"`
}

// MultiScrapedResult represents the result of scraping multiple URLs
type MultiScrapedResult struct {
	Results         []*ScrapedContent `json:"results"`
	SuccessCount    int               `json:"success_count"`
	FailureCount    int               `json:"failure_count"`
	TotalURLs       int               `json:"total_urls"`
	CombinedContent string            `json:"combined_content"`
	Timestamp       int64             `json:"timestamp"`
}

// YouTubeMetadata represents YouTube video metadata
type YouTubeMetadata struct {
	Title       string `json:"title"`
	Duration    string `json:"duration"`
	ViewCount   string `json:"view_count"`
	LikeCount   string `json:"like_count"`
	Channel     string `json:"channel"`
	Uploader    string `json:"uploader"`
	UploadDate  string `json:"upload_date"`
	Description string `json:"description"`
	Transcript  string `json:"transcript"`
}

// Scraper handles web scraping operations
type Scraper struct {
	client *http.Client
}

// NewScraper creates a new scraper instance
func NewScraper() *Scraper {
	return &Scraper{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ScrapeURL processes a URL and returns scraped content
func (s *Scraper) ScrapeURL(url string) (*ScrapedContent, error) {
	result := &ScrapedContent{
		URL:       url,
		Timestamp: time.Now().Unix(),
	}

	// Check if it's a YouTube URL
	if isYouTubeURL(url) {
		return s.scrapeYouTube(url, result)
	}

	// Regular web scraping
	return s.scrapeWebPage(url, result)
}

// ScrapeMultipleURLs processes multiple URLs concurrently
func (s *Scraper) ScrapeMultipleURLs(input string) (*MultiScrapedResult, error) {
	// Extract URLs from the input string
	urls := extractURLs(input)

	if len(urls) == 0 {
		return nil, fmt.Errorf("no valid URLs found in input")
	}

	result := &MultiScrapedResult{
		Results:   make([]*ScrapedContent, len(urls)),
		TotalURLs: len(urls),
		Timestamp: time.Now().Unix(),
	}

	// Use goroutines to scrape URLs concurrently
	var wg sync.WaitGroup
	for i, url := range urls {
		wg.Add(1)
		go func(index int, targetURL string) {
			defer wg.Done()

			scraped, err := s.ScrapeURL(targetURL)
			if err != nil {
				scraped = &ScrapedContent{
					URL:       targetURL,
					Error:     err.Error(),
					Timestamp: time.Now().Unix(),
				}
			}
			result.Results[index] = scraped
		}(i, url)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Process results and create combined content
	var combinedContent strings.Builder
	for _, scraped := range result.Results {
		if scraped.Error == "" {
			result.SuccessCount++
			combinedContent.WriteString(fmt.Sprintf("=== Content from %s ===\n", scraped.URL))
			combinedContent.WriteString(scraped.Content)
			combinedContent.WriteString("\n\n")
		} else {
			result.FailureCount++
		}
	}

	result.CombinedContent = combinedContent.String()
	return result, nil
}

// ProcessInput determines if input contains multiple URLs and processes accordingly
func (s *Scraper) ProcessInput(input string) (interface{}, error) {
	urls := extractURLs(input)

	if len(urls) == 0 {
		return nil, fmt.Errorf("no valid URLs found in input")
	}

	if len(urls) == 1 {
		// Single URL - use existing method
		return s.ScrapeURL(urls[0])
	}

	// Multiple URLs - use concurrent scraping
	return s.ScrapeMultipleURLs(input)
}

// scrapeYouTube extracts YouTube video metadata and transcript
func (s *Scraper) scrapeYouTube(url string, result *ScrapedContent) (*ScrapedContent, error) {
	// Get metadata using yt-dlp
	metadata, err := s.getYouTubeMetadata(url)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to get YouTube metadata: %v", err)
		return result, err
	}

	// Get transcript using yt-dlp
	transcript, err := s.getYouTubeTranscript(url)
	if err != nil {
		// Don't fail if transcript is not available
		transcript = ""
	}

	// Structure the content
	content := fmt.Sprintf("Title: %s\nChannel: %s\nDuration: %s\nView Count: %s\nDescription: %s\n\nTranscript:\n%s",
		metadata.Title, metadata.Channel, metadata.Duration, metadata.ViewCount, metadata.Description, transcript)

	result.Content = content
	result.Metadata = map[string]interface{}{
		"title":       metadata.Title,
		"channel":     metadata.Channel,
		"duration":    metadata.Duration,
		"view_count":  metadata.ViewCount,
		"like_count":  metadata.LikeCount,
		"uploader":    metadata.Uploader,
		"upload_date": metadata.UploadDate,
		"description": metadata.Description,
		"transcript":  transcript,
	}

	return result, nil
}

// scrapeWebPage extracts content from a regular web page
func (s *Scraper) scrapeWebPage(url string, result *ScrapedContent) (*ScrapedContent, error) {
	// Make HTTP request
	resp, err := s.client.Get(url)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to fetch URL: %v", err)
		return result, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		result.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status)
		return result, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// Parse HTML with goquery
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to parse HTML: %v", err)
		return result, err
	}

	// Remove script and style elements
	doc.Find("script, style").Remove()

	// Extract text content
	content := doc.Text()

	// Clean up whitespace
	content = cleanText(content)

	result.Content = content
	result.Metadata = map[string]interface{}{
		"title":       doc.Find("title").Text(),
		"description": doc.Find("meta[name=description]").AttrOr("content", ""),
		"url":         url,
	}

	return result, nil
}

// getYouTubeMetadata extracts metadata using yt-dlp
func (s *Scraper) getYouTubeMetadata(url string) (*YouTubeMetadata, error) {
	cmd := exec.Command("yt-dlp", "-j", url)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp metadata extraction failed: %v", err)
	}

	var rawMetadata map[string]interface{}
	if err := json.Unmarshal(output, &rawMetadata); err != nil {
		return nil, fmt.Errorf("failed to parse yt-dlp output: %v", err)
	}

	metadata := &YouTubeMetadata{}

	// Extract relevant fields with type safety
	if title, ok := rawMetadata["title"].(string); ok {
		metadata.Title = title
	}
	if duration, ok := rawMetadata["duration"].(float64); ok {
		metadata.Duration = formatDuration(int(duration))
	}
	if viewCount, ok := rawMetadata["view_count"].(float64); ok {
		metadata.ViewCount = strconv.FormatInt(int64(viewCount), 10)
	}
	if likeCount, ok := rawMetadata["like_count"].(float64); ok {
		metadata.LikeCount = strconv.FormatInt(int64(likeCount), 10)
	}
	if channel, ok := rawMetadata["channel"].(string); ok {
		metadata.Channel = channel
	}
	if uploader, ok := rawMetadata["uploader"].(string); ok {
		metadata.Uploader = uploader
	}
	if uploadDate, ok := rawMetadata["upload_date"].(string); ok {
		metadata.UploadDate = uploadDate
	}
	if description, ok := rawMetadata["description"].(string); ok {
		// Remove URLs from description
		urlRegex := regexp.MustCompile(`https?://\S+|www\.\S+`)
		metadata.Description = urlRegex.ReplaceAllString(description, "")
	}

	return metadata, nil
}

// getYouTubeTranscript extracts transcript using yt-dlp
func (s *Scraper) getYouTubeTranscript(url string) (string, error) {
	// Create temporary file for subtitle
	tmpFile, err := os.CreateTemp("", "transcript_*.srt")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer os.Remove(tmpFile.Name() + ".en.srt")

	baseFilePath := tmpFile.Name()

	// Run yt-dlp to download subtitles
	cmd := exec.Command("yt-dlp",
		"--skip-download",
		"--write-subs",
		"--write-auto-subs",
		"--sub-lang", "en",
		"--sub-format", "ttml",
		"--convert-subs", "srt",
		"--output", baseFilePath,
		url)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("yt-dlp subtitle extraction failed: %v", err)
	}

	// Read the generated subtitle file
	subtitlePath := baseFilePath + ".en.srt"
	content, err := os.ReadFile(subtitlePath)
	if err != nil {
		return "", fmt.Errorf("failed to read subtitle file: %v", err)
	}

	// Clean the transcript
	return cleanTranscript(string(content)), nil
}

// Helper functions

func isYouTubeURL(url string) bool {
	return strings.Contains(url, "youtube.com") || strings.Contains(url, "youtu.be")
}

func cleanText(text string) string {
	// Remove excessive whitespace
	re := regexp.MustCompile(`\s+`)
	return strings.TrimSpace(re.ReplaceAllString(text, " "))
}

func cleanTranscript(input string) string {
	lines := strings.Split(input, "\n")
	var cleanedLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip lines that contain only numbers
		if matched, _ := regexp.MatchString(`^\d+$`, line); matched {
			continue
		}

		// Skip timing lines
		if matched, _ := regexp.MatchString(`^\d{2}:\d{2}:\d{2}`, line); matched {
			continue
		}

		// Skip lines containing "-->"
		if strings.Contains(line, "-->") {
			continue
		}

		// Remove HTML tags
		htmlRegex := regexp.MustCompile(`<[^>]*>`)
		line = htmlRegex.ReplaceAllString(line, "")

		if line != "" {
			cleanedLines = append(cleanedLines, line)
		}
	}

	// Join and clean up spacing
	result := strings.Join(cleanedLines, " ")
	spaceRegex := regexp.MustCompile(`\s+`)
	return strings.TrimSpace(spaceRegex.ReplaceAllString(result, " "))
}

func formatDuration(seconds int) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, secs)
	}
	return fmt.Sprintf("%d:%02d", minutes, secs)
}

// extractURLs finds all URLs in the input string
func extractURLs(input string) []string {
	// URL regex pattern that matches http/https URLs - improved to handle YouTube URLs and query parameters
	urlRegex := regexp.MustCompile(`https?://(?:www\.)?[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-zA-Z0-9()]{1,6}\b(?:[-a-zA-Z0-9()@:%_\+.~#?&/=]*)?`)

	matches := urlRegex.FindAllString(input, -1)

	// Remove duplicates
	uniqueURLs := make([]string, 0, len(matches))
	seen := make(map[string]bool)

	for _, url := range matches {
		if !seen[url] {
			uniqueURLs = append(uniqueURLs, url)
			seen[url] = true
		}
	}

	return uniqueURLs
}
