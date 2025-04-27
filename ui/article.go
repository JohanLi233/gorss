package ui

import (
	"fmt"
	"strings"

	"github.com/JohanLi233/gorss/feed"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/net/html"
)

// ArticleView represents a view for displaying article content
type ArticleView struct {
	article      feed.Article
	width        int
	height       int
	scrollOffset int
	contentLines []string
}

// NewArticleView creates a new article view
func NewArticleView(article feed.Article, width, height int) *ArticleView {
	av := &ArticleView{
		article: article,
		width:   width,
		height:  height,
	}
	av.processContent()
	return av
}

// SetArticle updates the article being displayed
func (av *ArticleView) SetArticle(article feed.Article) {
	av.article = article
	av.scrollOffset = 0
	av.processContent()
}

// SetSize updates the width and height of the view
func (av *ArticleView) SetSize(width, height int) {
	av.width = width
	av.height = height
	av.processContent()
}

// processContent prepares the article content for display
func (av *ArticleView) processContent() {
	// If article is empty, return
	if av.article.Content == "" {
		av.contentLines = []string{"No content available"}
		return
	}

	// Clean HTML content
	content := cleanHTMLContent(av.article.Content)

	// Allow for title, metadata, and padding
	contentWidth := av.width - 6 // Account for padding and borders

	// Split the content into lines and handle wrapping
	lines := strings.Split(content, "\n")
	av.contentLines = []string{}

	for _, line := range lines {
		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			av.contentLines = append(av.contentLines, "")
			continue
		}

		// Handle line wrapping for long lines
		for len(line) > 0 {
			// Trim leading whitespace after wrapping
			line = strings.TrimLeftFunc(line, func(r rune) bool {
				return r == ' ' || r == '\t'
			})

			// If line is now empty after trimming, break
			if len(line) == 0 {
				break
			}

			if len(line) <= contentWidth {
				av.contentLines = append(av.contentLines, line)
				break
			}

			// Find a good breaking point
			wrapIdx := contentWidth
			for wrapIdx > 0 && !isBreakableChar(line[wrapIdx]) {
				wrapIdx--
			}

			// If we couldn't find a good break point, force a break
			if wrapIdx == 0 {
				wrapIdx = contentWidth
			}

			av.contentLines = append(av.contentLines, line[:wrapIdx])
			line = line[wrapIdx:]
		}
	}

	// Always ensure there's at least one line
	if len(av.contentLines) == 0 {
		av.contentLines = []string{"No content available"}
	}

	// Add some extra empty lines at the end to ensure scrolling works properly
	av.contentLines = append(av.contentLines, "", "", "", "", "")
}

// isBreakableChar checks if a character is suitable for line breaking
func isBreakableChar(c byte) bool {
	return c == ' ' || c == ',' || c == '.' || c == ';' || c == ':' || c == '-'
}

// cleanHTMLContent removes HTML tags from content
func cleanHTMLContent(content string) string {
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return content
	}

	var textContent strings.Builder
	extractText(doc, &textContent)

	return strings.TrimSpace(textContent.String())
}

// extractText recursively extracts text content from HTML nodes
func extractText(n *html.Node, sb *strings.Builder) {
	if n.Type == html.TextNode {
		sb.WriteString(n.Data)
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && (c.Data == "p" || c.Data == "div" || c.Data == "br") {
			// Add line breaks for block elements
			sb.WriteString("\n")
		}
		extractText(c, sb)
	}

	if n.Type == html.ElementNode && (n.Data == "p" || n.Data == "div") {
		// Add line break after block elements
		sb.WriteString("\n")
	}
}

// Render displays the article
func (av *ArticleView) Render() string {
	if av.article.Title == "" {
		return articleViewStyle.Width(av.width).Height(av.height).Render("No article selected")
	}

	// Render title
	title := articleTitleStyle.Render(av.article.Title)

	// Render metadata
	pubTime := av.article.Published.Format("2006-01-02 15:04")
	metadata := articleMetaStyle.Render(fmt.Sprintf("Published: %s | Source: %s", pubTime, av.article.FeedName))

	// Calculate available height for content - use exact height calculation
	headerHeight := lipgloss.Height(title) + lipgloss.Height(metadata) + 1 // +1 for margins
	contentHeight := av.height - headerHeight - 2                          // -2 for padding (reduced)

	// Ensure we have enough lines to display
	totalLines := len(av.contentLines)
	visibleLines := contentHeight
	maxOffset := totalLines - visibleLines

	// Adjust scroll offset if needed
	if maxOffset < 0 {
		maxOffset = 0
	}
	if av.scrollOffset > maxOffset {
		av.scrollOffset = maxOffset
	}

	// Get the visible portion of content
	visible := []string{}
	end := av.scrollOffset + visibleLines
	if end > totalLines {
		end = totalLines
	}

	for i := av.scrollOffset; i < end; i++ {
		if i < totalLines { // Make sure we don't go out of bounds
			visible = append(visible, av.contentLines[i])
		}
	}

	// If we have empty space at the bottom, fill it with more content if available
	if len(visible) < contentHeight && end < totalLines {
		additionalNeeded := contentHeight - len(visible)
		for i := 0; i < additionalNeeded && end+i < totalLines; i++ {
			visible = append(visible, av.contentLines[end+i])
		}
	}

	// Show end-of-feed marker when at last page
	end = av.scrollOffset + visibleLines
	if end > totalLines {
		end = totalLines
	}
	if end >= totalLines {
		if len(visible) == visibleLines {
			visible[visibleLines-1] = "--- End of Feed ---"
		} else {
			visible = append(visible, "", "--- End of Feed ---")
		}
	}

	// Join the visible lines
	content := strings.Join(visible, "\n")

	// No page information display

	// Combine everything
	full := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		metadata,
		content,
	)

	// Make sure we use Height() instead of MaxHeight() to use all available space
	return articleViewStyle.Width(av.width).Height(av.height).Render(full)
}

// ScrollUp moves the view up by one page
func (av *ArticleView) ScrollUp() {
	// Calculate visible height
	headerHeight := 2                             // Title + metadata + reduced margin
	contentHeight := av.height - headerHeight - 2 // -2 for padding (reduced)

	// Scroll by a page or less
	av.scrollOffset -= contentHeight
	if av.scrollOffset < 0 {
		av.scrollOffset = 0
	}
}

// ScrollDown moves the view down by one page
func (av *ArticleView) ScrollDown() {
	// Calculate visible height
	headerHeight := 2                             // Title + metadata + reduced margin
	contentHeight := av.height - headerHeight - 2 // -2 for padding (reduced)

	// Calculate maximum offset - extra safety to ensure we can scroll to the end
	totalLines := len(av.contentLines)
	maxOffset := totalLines - contentHeight + 5 // Added buffer to ensure we can reach the end

	// Scroll by a page or less
	av.scrollOffset += contentHeight / 2 // Scroll by half a page for better readability

	// Ensure limits
	if maxOffset < 0 {
		maxOffset = 0
	}
	if av.scrollOffset > maxOffset {
		av.scrollOffset = maxOffset
	}
	if av.scrollOffset < 0 {
		av.scrollOffset = 0
	}

	// Special handling for the last page - force scroll to very end
	if av.scrollOffset+contentHeight >= totalLines-5 {
		// We're getting close to the end, make sure we can reach it
		if totalLines > contentHeight {
			// Calculate the ideal offset to show the last line of content
			idealOffset := totalLines - contentHeight
			if av.scrollOffset < idealOffset {
				av.scrollOffset = idealOffset
			}
		}
	}
}
