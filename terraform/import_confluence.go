package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Configuration and data structures
type Config struct {
	ConfluenceURL    string `json:"CONFLUENCE_URL"`
	Username         string `json:"CONFLUENCE_USERNAME"`
	APIToken         string `json:"CONFLUENCE_API_TOKEN"`
	SpaceKeys        string `json:"space_keys"` // Comma-separated list of space keys
	SpaceKey         string `json:"space_key"`  // For backward compatibility
	IncludeBlogs     string `json:"include_blogs"`
	MaxWorkers       int    // Number of concurrent workers
	MaxContentLength int    // Maximum content length per page
	MaxPages         int    // Maximum number of pages to fetch (0 = unlimited)
}

type Page struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Type     string `json:"type"`
	SpaceKey string `json:"space_key"` // Add space key to track which space this page belongs to
}

type PagesResponse struct {
	Results []Page `json:"results"`
	Links   struct {
		Next string `json:"next"`
	} `json:"_links"`
}

type ContentResponse struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Body  struct {
		Storage struct {
			Value string `json:"value"`
		} `json:"storage"`
	} `json:"body"`
	Metadata struct {
		Labels struct {
			Results []struct {
				Name string `json:"name"`
			} `json:"results"`
		} `json:"labels"`
	} `json:"metadata"`
}

type ProcessedItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Content  string `json:"content"`
	Type     string `json:"type"`
	Labels   string `json:"labels"`
	SpaceKey string `json:"space_key"` // Add space key to track which space this item belongs to
}

type Result struct {
	Items string `json:"items"`
	Error string `json:"error,omitempty"`
}

// HTTP client with connection pooling
var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	},
}

// HTML to text conversion with better performance
type HTMLConverter struct {
	// Pre-compiled regular expressions for better performance
	tableRegex        *regexp.Regexp
	rowRegex          *regexp.Regexp
	cellRegex         *regexp.Regexp
	tagRegex          *regexp.Regexp
	headerRegexes     map[int]*regexp.Regexp
	listRegexes       map[string]*regexp.Regexp
	formatRegexes     map[string]*regexp.Regexp
	linkRegex         *regexp.Regexp
	entityMap         map[string]string
	multiNewlineRegex *regexp.Regexp
	multiSpaceRegex   *regexp.Regexp
}

func NewHTMLConverter() *HTMLConverter {
	return &HTMLConverter{
		tableRegex: regexp.MustCompile(`(?i)<table[^>]*>.*?</table>`),
		rowRegex:   regexp.MustCompile(`(?i)<tr[^>]*>(.*?)</tr>`),
		cellRegex:  regexp.MustCompile(`(?i)<(?:th|td)[^>]*>(.*?)</(?:th|td)>`),
		tagRegex:   regexp.MustCompile(`<[^>]+>`),
		headerRegexes: map[int]*regexp.Regexp{
			1: regexp.MustCompile(`(?i)<h1[^>]*>(.*?)</h1>`),
			2: regexp.MustCompile(`(?i)<h2[^>]*>(.*?)</h2>`),
			3: regexp.MustCompile(`(?i)<h3[^>]*>(.*?)</h3>`),
			4: regexp.MustCompile(`(?i)<h4[^>]*>(.*?)</h4>`),
			5: regexp.MustCompile(`(?i)<h5[^>]*>(.*?)</h5>`),
			6: regexp.MustCompile(`(?i)<h6[^>]*>(.*?)</h6>`),
		},
		listRegexes: map[string]*regexp.Regexp{
			"ul_start": regexp.MustCompile(`(?i)<ul[^>]*>`),
			"ul_end":   regexp.MustCompile(`(?i)</ul>`),
			"ol_start": regexp.MustCompile(`(?i)<ol[^>]*>`),
			"ol_end":   regexp.MustCompile(`(?i)</ol>`),
			"li":       regexp.MustCompile(`(?i)<li[^>]*>(.*?)</li>`),
		},
		formatRegexes: map[string]*regexp.Regexp{
			"strong": regexp.MustCompile(`(?i)<strong[^>]*>(.*?)</strong>`),
			"b":      regexp.MustCompile(`(?i)<b[^>]*>(.*?)</b>`),
			"em":     regexp.MustCompile(`(?i)<em[^>]*>(.*?)</em>`),
			"i":      regexp.MustCompile(`(?i)<i[^>]*>(.*?)</i>`),
			"u":      regexp.MustCompile(`(?i)<u[^>]*>(.*?)</u>`),
			"code":   regexp.MustCompile(`(?i)<code[^>]*>(.*?)</code>`),
			"pre":    regexp.MustCompile(`(?is)<pre[^>]*>(.*?)</pre>`),
			"p":      regexp.MustCompile(`(?i)<p[^>]*>(.*?)</p>`),
			"div":    regexp.MustCompile(`(?i)<div[^>]*>(.*?)</div>`),
			"br":     regexp.MustCompile(`(?i)<br[^>]*>`),
		},
		linkRegex: regexp.MustCompile(`(?i)<a[^>]*href="([^"]*)"[^>]*>(.*?)</a>`),
		entityMap: map[string]string{
			"&nbsp;":   " ",
			"&lt;":     "<",
			"&gt;":     ">",
			"&amp;":    "&",
			"&quot;":   "\"",
			"&apos;":   "'",
			"&ldquo;":  "\"",
			"&rdquo;":  "\"",
			"&lsquo;":  "'",
			"&rsquo;":  "'",
			"&mdash;":  "—",
			"&ndash;":  "–",
			"&rarr;":   "→",
			"&larr;":   "←",
			"&uarr;":   "↑",
			"&darr;":   "↓",
			"&hellip;": "...",
		},
		multiNewlineRegex: regexp.MustCompile(`\n{3,}`),
		multiSpaceRegex:   regexp.MustCompile(` +`),
	}
}

func (h *HTMLConverter) convertHTMLTable(tableHTML string) string {
	rows := h.rowRegex.FindAllStringSubmatch(tableHTML, -1)
	if len(rows) == 0 {
		return "\n[Empty table]\n"
	}

	var markdownRows []string
	isHeader := true

	for _, row := range rows {
		if len(row) < 2 {
			continue
		}

		cells := h.cellRegex.FindAllStringSubmatch(row[1], -1)
		if len(cells) == 0 {
			continue
		}

		var cleanCells []string
		for _, cell := range cells {
			if len(cell) < 2 {
				cleanCells = append(cleanCells, " ")
				continue
			}

			// Clean cell content
			cleanCell := h.tagRegex.ReplaceAllString(cell[1], " ")

			// Replace HTML entities
			for entity, replacement := range h.entityMap {
				cleanCell = strings.ReplaceAll(cleanCell, entity, replacement)
			}

			// Normalize whitespace
			cleanCell = strings.TrimSpace(h.multiSpaceRegex.ReplaceAllString(cleanCell, " "))

			// Escape pipe characters
			cleanCell = strings.ReplaceAll(cleanCell, "|", "\\|")

			if cleanCell == "" {
				cleanCell = " "
			}
			cleanCells = append(cleanCells, cleanCell)
		}

		// Format as markdown table row
		markdownRow := "| " + strings.Join(cleanCells, " | ") + " |"
		markdownRows = append(markdownRows, markdownRow)

		// Add header separator after first row
		if isHeader && len(cleanCells) > 0 {
			separator := "|" + strings.Repeat(" --- |", len(cleanCells))
			markdownRows = append(markdownRows, separator)
			isHeader = false
		}
	}

	if len(markdownRows) > 0 {
		return "\n\n" + strings.Join(markdownRows, "\n") + "\n\n"
	}
	return "\n[Empty table]\n"
}

func (h *HTMLConverter) htmlToText(htmlContent string) string {
	// Handle special Confluence macros
	htmlContent = regexp.MustCompile(`(?i)<ac:link[^>]*>.*?</ac:link>`).ReplaceAllString(htmlContent, "")

	// Handle headers
	for level, regex := range h.headerRegexes {
		prefix := strings.Repeat("#", level)
		htmlContent = regex.ReplaceAllString(htmlContent, "\n\n"+prefix+" $1\n\n")
	}

	// Handle lists
	htmlContent = h.listRegexes["ul_start"].ReplaceAllString(htmlContent, "\n")
	htmlContent = h.listRegexes["ul_end"].ReplaceAllString(htmlContent, "\n")
	htmlContent = h.listRegexes["ol_start"].ReplaceAllString(htmlContent, "\n")
	htmlContent = h.listRegexes["ol_end"].ReplaceAllString(htmlContent, "\n")
	htmlContent = h.listRegexes["li"].ReplaceAllString(htmlContent, "- $1\n")

	// Handle text formatting
	htmlContent = h.formatRegexes["strong"].ReplaceAllString(htmlContent, "**$1**")
	htmlContent = h.formatRegexes["b"].ReplaceAllString(htmlContent, "**$1**")
	htmlContent = h.formatRegexes["em"].ReplaceAllString(htmlContent, "*$1*")
	htmlContent = h.formatRegexes["i"].ReplaceAllString(htmlContent, "*$1*")
	htmlContent = h.formatRegexes["u"].ReplaceAllString(htmlContent, "_$1_")
	htmlContent = h.formatRegexes["code"].ReplaceAllString(htmlContent, "`$1`")
	htmlContent = h.formatRegexes["pre"].ReplaceAllString(htmlContent, "```\n$1\n```")

	// Handle paragraphs and divs
	htmlContent = h.formatRegexes["p"].ReplaceAllString(htmlContent, "\n\n$1\n\n")
	htmlContent = h.formatRegexes["div"].ReplaceAllString(htmlContent, "\n$1\n")
	htmlContent = h.formatRegexes["br"].ReplaceAllString(htmlContent, "\n")

	// Handle links
	htmlContent = h.linkRegex.ReplaceAllString(htmlContent, "[$2]($1)")

	// Handle tables
	htmlContent = h.tableRegex.ReplaceAllStringFunc(htmlContent, h.convertHTMLTable)

	// Remove remaining HTML tags
	htmlContent = h.tagRegex.ReplaceAllString(htmlContent, " ")

	// Replace HTML entities
	for entity, replacement := range h.entityMap {
		htmlContent = strings.ReplaceAll(htmlContent, entity, replacement)
	}

	// Clean up whitespace
	htmlContent = h.multiNewlineRegex.ReplaceAllString(htmlContent, "\n\n")
	htmlContent = h.multiSpaceRegex.ReplaceAllString(htmlContent, " ")
	htmlContent = strings.TrimSpace(htmlContent)

	return htmlContent
}

// HTTP request helper
func makeRequest(url, username, apiToken string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Set authorization header
	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + apiToken))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	return body, nil
}

// Fetch all pages with pagination from multiple spaces
func fetchAllPages(config *Config) ([]Page, error) {
	// Parse space keys - support both comma-separated list and single space key for backward compatibility
	var spaceKeys []string
	if config.SpaceKeys != "" {
		spaceKeys = strings.Split(strings.TrimSpace(config.SpaceKeys), ",")
		for i, key := range spaceKeys {
			spaceKeys[i] = strings.TrimSpace(key)
		}
	} else if config.SpaceKey != "" {
		// Backward compatibility
		spaceKeys = []string{strings.TrimSpace(config.SpaceKey)}
	}

	if len(spaceKeys) == 0 {
		return nil, fmt.Errorf("no space keys provided")
	}

	fmt.Fprintf(os.Stderr, "DEBUG: Processing %d space(s): %v (max pages per space: %d)\n", len(spaceKeys), spaceKeys, config.MaxPages)

	var allPages []Page
	pagesPerSpace := config.MaxPages

	// If we have multiple spaces and a max_pages limit, distribute the limit across spaces
	if len(spaceKeys) > 1 && config.MaxPages > 0 {
		pagesPerSpace = config.MaxPages / len(spaceKeys)
		if pagesPerSpace == 0 {
			pagesPerSpace = 1 // Ensure at least 1 page per space
		}
		fmt.Fprintf(os.Stderr, "DEBUG: Limiting to %d pages per space (total limit: %d)\n", pagesPerSpace, config.MaxPages)
	}

	// Process each space
	for spaceIndex, spaceKey := range spaceKeys {
		fmt.Fprintf(os.Stderr, "DEBUG: Processing space %d/%d: %s\n", spaceIndex+1, len(spaceKeys), spaceKey)

		// First, get the space ID from the space key
		spaceInfoURL := fmt.Sprintf("%s/api/v2/spaces?keys=%s", strings.TrimSuffix(config.ConfluenceURL, "/"), spaceKey)
		fmt.Fprintf(os.Stderr, "DEBUG: Getting space ID from: %s\n", spaceInfoURL)

		spaceBody, err := makeRequest(spaceInfoURL, config.Username, config.APIToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "DEBUG: Failed to get space info for %s: %v\n", spaceKey, err)
			continue // Skip this space and continue with others
		}

		var spaceResponse struct {
			Results []struct {
				ID  string `json:"id"`
				Key string `json:"key"`
			} `json:"results"`
		}

		if err := json.Unmarshal(spaceBody, &spaceResponse); err != nil {
			fmt.Fprintf(os.Stderr, "DEBUG: Failed to parse space response for %s: %v\n", spaceKey, err)
			continue
		}

		if len(spaceResponse.Results) == 0 {
			fmt.Fprintf(os.Stderr, "DEBUG: Space not found: %s\n", spaceKey)
			continue
		}

		spaceID := spaceResponse.Results[0].ID
		fmt.Fprintf(os.Stderr, "DEBUG: Found space ID: %s for space key: %s\n", spaceID, spaceKey)

		var spacePages []Page
		endpoint := fmt.Sprintf("/api/v2/spaces/%s/pages?limit=100", spaceID)
		pagesFromSpace := 0

		fmt.Fprintf(os.Stderr, "DEBUG: Using API endpoint pattern: /api/v2/spaces/%s/pages (same as bash script)\n", spaceID)

		for endpoint != "" {
			// Check if we've reached the limit for this space
			if pagesPerSpace > 0 && pagesFromSpace >= pagesPerSpace {
				fmt.Fprintf(os.Stderr, "DEBUG: Reached max pages limit (%d) for space %s, stopping fetch\n", pagesPerSpace, spaceKey)
				break
			}

			fullURL := strings.TrimSuffix(config.ConfluenceURL, "/") + endpoint
			fmt.Fprintf(os.Stderr, "DEBUG: Fetching %s\n", fullURL)

			body, err := makeRequest(fullURL, config.Username, config.APIToken)
			if err != nil {
				fmt.Fprintf(os.Stderr, "DEBUG: Failed to fetch pages from space %s: %v\n", spaceKey, err)
				break
			}

			var response PagesResponse
			if err := json.Unmarshal(body, &response); err != nil {
				fmt.Fprintf(os.Stderr, "DEBUG: Failed to parse response for space %s: %v\n", spaceKey, err)
				break
			}

			// Debug: Show what types of content we're getting
			if len(response.Results) > 0 {
				typeCount := make(map[string]int)
				for _, page := range response.Results {
					if page.Type == "" {
						typeCount["page"] = typeCount["page"] + 1 // Default to page if empty
					} else {
						typeCount[page.Type]++
					}
				}
				fmt.Fprintf(os.Stderr, "DEBUG: Content types in this batch from space %s: %+v\n", spaceKey, typeCount)

				// Show a few example titles
				fmt.Fprintf(os.Stderr, "DEBUG: Example titles in this batch from space %s:\n", spaceKey)
				for i, page := range response.Results[:min(3, len(response.Results))] {
					pageType := page.Type
					if pageType == "" {
						pageType = "page"
					}
					fmt.Fprintf(os.Stderr, "  %d. [%s] %s (ID: %s)\n", i+1, pageType, page.Title, page.ID)
				}
			}

			// Add results, but respect the limit and set space key
			pagesToAdd := response.Results
			if pagesPerSpace > 0 {
				remaining := pagesPerSpace - pagesFromSpace
				if len(pagesToAdd) > remaining {
					pagesToAdd = pagesToAdd[:remaining]
					fmt.Fprintf(os.Stderr, "DEBUG: Limiting to %d pages to stay within space limit for %s\n", remaining, spaceKey)
				}
			}

			// Set space key for each page
			for i := range pagesToAdd {
				pagesToAdd[i].SpaceKey = spaceKey
			}

			spacePages = append(spacePages, pagesToAdd...)
			pagesFromSpace += len(pagesToAdd)
			fmt.Fprintf(os.Stderr, "DEBUG: Fetched %d pages from space %s, total from this space: %d\n", len(pagesToAdd), spaceKey, pagesFromSpace)

			// Stop if we've reached the limit for this space
			if pagesPerSpace > 0 && pagesFromSpace >= pagesPerSpace {
				fmt.Fprintf(os.Stderr, "DEBUG: Reached max pages limit (%d) for space %s, stopping\n", pagesPerSpace, spaceKey)
				break
			}

			// Get next endpoint - handle cursor-based pagination
			if response.Links.Next != "" {
				if strings.HasPrefix(response.Links.Next, "/wiki/") {
					endpoint = response.Links.Next[5:] // Remove "/wiki" prefix
				} else {
					endpoint = response.Links.Next
				}
				fmt.Fprintf(os.Stderr, "DEBUG: Next endpoint for space %s: %s\n", spaceKey, endpoint)
			} else {
				endpoint = ""
			}
		}

		// Add pages from this space to the overall collection
		allPages = append(allPages, spacePages...)
		fmt.Fprintf(os.Stderr, "DEBUG: Completed space %s: %d pages, total so far: %d\n", spaceKey, len(spacePages), len(allPages))
	}

	// Final summary of content types across all spaces
	finalTypeCount := make(map[string]int)
	spaceCount := make(map[string]int)
	for _, page := range allPages {
		if page.Type == "" {
			finalTypeCount["page"] = finalTypeCount["page"] + 1
		} else {
			finalTypeCount[page.Type]++
		}
		spaceCount[page.SpaceKey]++
	}
	fmt.Fprintf(os.Stderr, "DEBUG: Final content type breakdown across all spaces: %+v\n", finalTypeCount)
	fmt.Fprintf(os.Stderr, "DEBUG: Pages per space: %+v\n", spaceCount)
	fmt.Fprintf(os.Stderr, "DEBUG: Total pages fetched from all spaces: %d\n", len(allPages))
	return allPages, nil
}

// Worker function to process pages concurrently
func pageWorker(config *Config, converter *HTMLConverter, pages <-chan Page, results chan<- *ProcessedItem, wg *sync.WaitGroup) {
	defer wg.Done()

	for page := range pages {
		// Get full page content using v1 API
		contentURL := fmt.Sprintf("%s/rest/api/content/%s?expand=body.storage,metadata.labels",
			strings.TrimSuffix(config.ConfluenceURL, "/"), page.ID)

		body, err := makeRequest(contentURL, config.Username, config.APIToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "DEBUG: Failed to get content for page %s from space %s: %v\n", page.Title, page.SpaceKey, err)
			continue
		}

		var contentResponse ContentResponse
		if err := json.Unmarshal(body, &contentResponse); err != nil {
			fmt.Fprintf(os.Stderr, "DEBUG: Failed to parse content response for page %s from space %s: %v\n", page.Title, page.SpaceKey, err)
			continue
		}

		// Convert HTML to text
		cleanContent := converter.htmlToText(contentResponse.Body.Storage.Value)

		// Skip empty pages
		if strings.TrimSpace(cleanContent) == "" {
			fmt.Fprintf(os.Stderr, "DEBUG: Skipping empty page: %s from space %s\n", page.Title, page.SpaceKey)
			continue
		}

		// Limit content size
		if len(cleanContent) > config.MaxContentLength {
			fmt.Fprintf(os.Stderr, "DEBUG: Truncating large content for page: %s from space %s (%d chars)\n", page.Title, page.SpaceKey, len(cleanContent))
			cleanContent = cleanContent[:config.MaxContentLength] + "\n\n[Content truncated due to size limits]"
		}

		// Extract labels
		var labels []string
		for _, label := range contentResponse.Metadata.Labels.Results {
			labels = append(labels, label.Name)
		}

		// Determine content type
		contentType := "page"
		if page.Type == "blogpost" {
			contentType = "blog"
		}

		item := &ProcessedItem{
			ID:       contentResponse.ID,
			Title:    contentResponse.Title,
			Content:  cleanContent,
			Type:     contentType,
			Labels:   strings.Join(labels, ","),
			SpaceKey: page.SpaceKey,
		}

		results <- item
		fmt.Fprintf(os.Stderr, "DEBUG: Added page: %s from space %s (content length: %d)\n", page.Title, page.SpaceKey, len(cleanContent))
	}
}

func main() {
	// Read input from stdin
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		result := Result{Error: fmt.Sprintf("Failed to read input: %v", err)}
		json.NewEncoder(os.Stdout).Encode(result)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "DEBUG: Received input: %s...\n", string(input)[:min(200, len(input))])

	// Parse JSON input as map first to handle max_pages parameter
	var inputMap map[string]interface{}
	if err := json.Unmarshal(input, &inputMap); err != nil {
		result := Result{Error: fmt.Sprintf("Failed to parse input JSON: %v", err)}
		json.NewEncoder(os.Stdout).Encode(result)
		os.Exit(1)
	}

	// Parse configuration
	var config Config
	if err := json.Unmarshal(input, &config); err != nil {
		result := Result{Error: fmt.Sprintf("Failed to parse input JSON: %v", err)}
		json.NewEncoder(os.Stdout).Encode(result)
		os.Exit(1)
	}

	// Set defaults
	if config.MaxWorkers == 0 {
		config.MaxWorkers = 5 // Concurrent workers for page processing
	}
	if config.MaxContentLength == 0 {
		config.MaxContentLength = 250000
	}
	// Parse MaxPages from input - if not provided, default to 0 (unlimited)
	if maxPagesValue, exists := inputMap["max_pages"]; exists {
		if maxPagesStr, ok := maxPagesValue.(string); ok && maxPagesStr != "" {
			if maxPages, err := strconv.Atoi(maxPagesStr); err == nil && maxPages > 0 {
				config.MaxPages = maxPages
			}
		}
	}

	// Debug parameter values
	fmt.Fprintf(os.Stderr, "DEBUG: Parameters received:\n")
	fmt.Fprintf(os.Stderr, "  CONFLUENCE_URL: %s\n", config.ConfluenceURL)
	fmt.Fprintf(os.Stderr, "  CONFLUENCE_USERNAME: %s\n", config.Username)
	fmt.Fprintf(os.Stderr, "  CONFLUENCE_API_TOKEN: %s\n", func() string {
		if config.APIToken != "" {
			return "***"
		}
		return "EMPTY"
	}())
	fmt.Fprintf(os.Stderr, "  space_keys: %s\n", config.SpaceKeys)
	fmt.Fprintf(os.Stderr, "  space_key (legacy): %s\n", config.SpaceKey)
	fmt.Fprintf(os.Stderr, "  include_blogs: %s\n", config.IncludeBlogs)
	fmt.Fprintf(os.Stderr, "  max_pages: %d\n", config.MaxPages)
	fmt.Fprintf(os.Stderr, "  max_workers: %d\n", config.MaxWorkers)

	// Check for required parameters
	var missingParams []string
	if config.ConfluenceURL == "" {
		missingParams = append(missingParams, "CONFLUENCE_URL")
	}
	if config.Username == "" {
		missingParams = append(missingParams, "CONFLUENCE_USERNAME")
	}
	if config.APIToken == "" {
		missingParams = append(missingParams, "CONFLUENCE_API_TOKEN")
	}
	if config.SpaceKeys == "" && config.SpaceKey == "" {
		missingParams = append(missingParams, "space_keys or space_key")
	}

	// If all required parameters are empty, Confluence is disabled - return empty results
	if config.ConfluenceURL == "" && config.Username == "" && config.APIToken == "" && config.SpaceKeys == "" && config.SpaceKey == "" {
		fmt.Fprintf(os.Stderr, "DEBUG: Confluence is disabled - returning empty results\n")
		result := Result{Items: "[]"}
		json.NewEncoder(os.Stdout).Encode(result)
		os.Exit(0)
	}

	if len(missingParams) > 0 {
		errorMsg := fmt.Sprintf("Missing required parameters: %s", strings.Join(missingParams, ", "))
		fmt.Fprintf(os.Stderr, "DEBUG: %s\n", errorMsg)
		result := Result{Error: errorMsg}
		json.NewEncoder(os.Stdout).Encode(result)
		os.Exit(1)
	}

	// Test connection
	testURL := fmt.Sprintf("%s/api/v2/pages?limit=1", strings.TrimSuffix(config.ConfluenceURL, "/"))
	fmt.Fprintf(os.Stderr, "DEBUG: Testing connection to: %s\n", testURL)

	_, err = makeRequest(testURL, config.Username, config.APIToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "DEBUG: Connection test failed: %v\n", err)
		result := Result{Error: fmt.Sprintf("Confluence connection failed: %v", err)}
		json.NewEncoder(os.Stdout).Encode(result)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "DEBUG: Connection test successful\n")

	// Fetch all pages
	pages, err := fetchAllPages(&config)
	if err != nil {
		result := Result{Error: fmt.Sprintf("Failed to fetch pages: %v", err)}
		json.NewEncoder(os.Stdout).Encode(result)
		os.Exit(1)
	}

	// Create HTML converter
	converter := NewHTMLConverter()

	// Set up concurrent processing
	pagesChan := make(chan Page, 100)
	resultsChan := make(chan *ProcessedItem, 100)
	var wg sync.WaitGroup

	// Start worker goroutines
	for i := 0; i < config.MaxWorkers; i++ {
		wg.Add(1)
		go pageWorker(&config, converter, pagesChan, resultsChan, &wg)
	}

	// Start result collector goroutine
	var items []*ProcessedItem
	var resultWg sync.WaitGroup
	resultWg.Add(1)
	go func() {
		defer resultWg.Done()
		for item := range resultsChan {
			items = append(items, item)
		}
	}()

	// Send pages to workers
	go func() {
		defer close(pagesChan)
		for _, page := range pages {
			pagesChan <- page
		}
	}()

	// Wait for all workers to complete
	wg.Wait()
	close(resultsChan)

	// Wait for result collector
	resultWg.Wait()

	fmt.Fprintf(os.Stderr, "DEBUG: Final item count: %d\n", len(items))

	// Convert items to JSON string
	itemsJSON, err := json.Marshal(items)
	if err != nil {
		result := Result{Error: fmt.Sprintf("Failed to marshal items: %v", err)}
		json.NewEncoder(os.Stdout).Encode(result)
		os.Exit(1)
	}

	// Return result
	result := Result{Items: string(itemsJSON)}
	json.NewEncoder(os.Stdout).Encode(result)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
