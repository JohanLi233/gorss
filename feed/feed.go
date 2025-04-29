package feed

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/JohanLi233/gorss/config"
	"github.com/mmcdole/gofeed"
)

// Article represents a single RSS article
type Article struct {
	Title       string
	Description string
	Content     string
	Link        string
	Published   time.Time
	FeedName    string
}

// FeedSummary contains the LLM-generated summary for a feed
type FeedSummary struct {
	FeedName    string
	Summary     string
	Generated   time.Time
	ArticleCount int
}

// FeedManager handles fetching and storing feed data
type FeedManager struct {
	Feeds     []config.Feed
	Articles  []Article
	Summaries map[string]FeedSummary // Key is feed name
	parser    *gofeed.Parser
	mu        sync.RWMutex
	cachePath string
	summaryPath string
}

// NewFeedManager creates a new feed manager and loads cached articles if available
func NewFeedManager(feeds []config.Feed) *FeedManager {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	cachePath := filepath.Join(homeDir, ".cache", "gorss", "feed_cache.json")
	summaryPath := filepath.Join(homeDir, ".cache", "gorss", "summaries.json")

	fm := &FeedManager{
		Feeds:       feeds,
		Summaries:   make(map[string]FeedSummary),
		parser:      gofeed.NewParser(),
		cachePath:   cachePath,
		summaryPath: summaryPath,
	}

	// Load feed cache
	if err := fm.loadCache(); err != nil {
		fmt.Printf("Warning: failed to load feed cache: %v\n", err)
	}

	// Load summaries cache
	if err := fm.loadSummaries(); err != nil {
		fmt.Printf("Warning: failed to load summaries cache: %v\n", err)
	}

	return fm
}

// RefreshFeeds fetches the latest articles from all configured feeds
func (fm *FeedManager) RefreshFeeds() error {
	var wg sync.WaitGroup
	articleCh := make(chan []Article, len(fm.Feeds))
	errorCh := make(chan error, len(fm.Feeds))

	for _, feed := range fm.Feeds {
		// Skip the "All" feed since it's just a category, not a real feed
		if feed.URL == "" {
			continue
		}

		wg.Add(1)
		go func(feed config.Feed) {
			defer wg.Done()
			articles, err := fm.fetchFeed(feed)
			if err != nil {
				errorCh <- fmt.Errorf("failed to fetch %s: %w", feed.Name, err)
				return
			}
			articleCh <- articles
		}(feed)
	}

	go func() {
		wg.Wait()
		close(articleCh)
		close(errorCh)
	}()

	var newArticles []Article
	for articles := range articleCh {
		newArticles = append(newArticles, articles...)
	}

	var errors []error
	for err := range errorCh {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		// Return the first error but continue with any successfully fetched feeds
		return errors[0]
	}

	fm.mu.Lock()
	fm.Articles = newArticles
	fm.mu.Unlock()

	if err := fm.saveCache(); err != nil {
		fmt.Printf("Warning: failed to save feed cache: %v\n", err)
	}

	return nil
}

// GetArticles returns a copy of all articles
func (fm *FeedManager) GetArticles() []Article {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	// Return a copy to avoid race conditions
	result := make([]Article, len(fm.Articles))
	copy(result, fm.Articles)
	return result
}

// fetchFeed fetches a single feed and converts it to articles
func (fm *FeedManager) fetchFeed(feed config.Feed) ([]Article, error) {
	parsed, err := fm.parser.ParseURL(feed.URL)
	if err != nil {
		return nil, err
	}

	var articles []Article
	for _, item := range parsed.Items {
		pubDate := time.Now()
		if item.PublishedParsed != nil {
			pubDate = *item.PublishedParsed
		}

		content := item.Content
		if content == "" {
			content = item.Description
		}

		articles = append(articles, Article{
			Title:       item.Title,
			Description: item.Description,
			Content:     content,
			Link:        item.Link,
			Published:   pubDate,
			FeedName:    feed.Name,
		})
	}

	return articles, nil
}

// loadCache loads cached articles from the cache file
func (fm *FeedManager) loadCache() error {
	data, err := os.ReadFile(fm.cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var articles []Article
	if err := json.Unmarshal(data, &articles); err != nil {
		return err
	}
	fm.mu.Lock()
	fm.Articles = articles
	fm.mu.Unlock()
	return nil
}

// saveCache saves current articles to the cache file
func (fm *FeedManager) saveCache() error {
	dir := filepath.Dir(fm.cachePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	fm.mu.RLock()
	data, err := json.MarshalIndent(fm.Articles, "", "  ")
	fm.mu.RUnlock()
	if err != nil {
		return err
	}
	if err := os.WriteFile(fm.cachePath, data, 0644); err != nil {
		return err
	}
	return nil
}

// GetSummary returns the summary for a specific feed
func (fm *FeedManager) GetSummary(feedName string) (FeedSummary, bool) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	summary, exists := fm.Summaries[feedName]
	return summary, exists
}

// SetSummary sets or updates the summary for a feed
func (fm *FeedManager) SetSummary(feedName string, summary string, articleCount int) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	fm.Summaries[feedName] = FeedSummary{
		FeedName:     feedName,
		Summary:      summary,
		Generated:    time.Now(),
		ArticleCount: articleCount,
	}

	// Save summaries to disk
	_ = fm.saveSummaries()
}

// loadSummaries loads cached summaries from the summary file
func (fm *FeedManager) loadSummaries() error {
	data, err := os.ReadFile(fm.summaryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var summaries map[string]FeedSummary
	if err := json.Unmarshal(data, &summaries); err != nil {
		return err
	}

	fm.mu.Lock()
	fm.Summaries = summaries
	fm.mu.Unlock()
	return nil
}

// saveSummaries saves current summaries to the summary file
func (fm *FeedManager) saveSummaries() error {
	dir := filepath.Dir(fm.summaryPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	fm.mu.RLock()
	data, err := json.MarshalIndent(fm.Summaries, "", "  ")
	fm.mu.RUnlock()

	if err != nil {
		return err
	}

	if err := os.WriteFile(fm.summaryPath, data, 0644); err != nil {
		return err
	}

	return nil
}
