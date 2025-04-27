package ui

import (
	"fmt"
	"io"

	"github.com/JohanLi233/gorss/feed"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// Item represents a selectable list item
type Item struct {
	title       string
	description string
	data        interface{}
}

// FilterValue implements list.Item
func (i Item) FilterValue() string { return i.title }

// Title returns the item's title
func (i Item) Title() string { return i.title }

// Description returns the item's description
func (i Item) Description() string { return i.description }

// NewArticleItem creates a new list item from an article
func NewArticleItem(article feed.Article) Item {
	pubTime := article.Published.Format("2006-01-02 15:04")
	return Item{
		title:       article.Title,
		description: fmt.Sprintf("[%s] %s", pubTime, article.FeedName),
		data:        article,
	}
}

// NewFeedItem creates a new list item from a feed name
func NewFeedItem(name string) Item {
	return Item{
		title:       name,
		description: "RSS Feed",
		data:        name,
	}
}

// ItemDelegate is a delegate for rendering list items
type ItemDelegate struct{}

// Height returns the height of a list item
func (d ItemDelegate) Height() int { return 2 }

// Spacing returns the spacing between list items
func (d ItemDelegate) Spacing() int { return 1 }

// Update handles updating the list items
func (d ItemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

// Render renders a list item
func (d ItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(Item)
	if !ok {
		return
	}

	str := fmt.Sprintf("%s\n%s", i.title, i.description)

	fn := normalArticleStyle.Render
	if index == m.Index() {
		fn = selectedArticleStyle.Render
	}

	fmt.Fprint(w, fn(str))
}

// CreateFeedsList creates a new list for feeds
func CreateFeedsList(feeds []string, width, height int) list.Model {
	var items []list.Item
	for _, f := range feeds {
		items = append(items, NewFeedItem(f))
	}

	l := list.New(items, ItemDelegate{}, width, height)
	l.Title = "Feeds"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = helpStyle
	l.Styles.HelpStyle = helpStyle

	return l
}

// CreateArticlesList creates a new list for articles
func CreateArticlesList(articles []feed.Article, width, height int) list.Model {
	var items []list.Item
	for _, a := range articles {
		items = append(items, NewArticleItem(a))
	}

	l := list.New(items, ItemDelegate{}, width, height)
	l.Title = "Articles"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = helpStyle
	l.Styles.HelpStyle = helpStyle

	return l
}
