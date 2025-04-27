package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lizhonghan/gorss/config"
	"github.com/lizhonghan/gorss/feed"
)

// View states
const (
	viewFeeds = iota
	viewArticles
	viewArticleDetail
	viewConfig // 配置视图
)

// Messages
type fetchCompleteMsg struct{ err error }
type fetchStartMsg struct{}
type saveConfigCompleteMsg struct{ err error }
type exitConfigMsg struct{}

// Model represents the main application UI model
type Model struct {
	feedManager   *feed.FeedManager
	feeds         []string
	feedsList     list.Model
	articlesList  list.Model
	articleView   *ArticleView
	configView    *ConfigView // 配置视图
	currentView   int
	currentFeed   string
	width         int
	height        int
	ready         bool
	loading       bool
	errorMessage  string
	statusMessage string
}

// NewModel creates a new application model
func NewModel(feedManager *feed.FeedManager) Model {
	var feeds []string
	for _, f := range feedManager.Feeds {
		feeds = append(feeds, f.Name)
	}

	return Model{
		feedManager: feedManager,
		feeds:       feeds,
		currentView: viewFeeds,
		currentFeed: feeds[0],
		loading:     true,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		func() tea.Msg {
			return fetchStartMsg{}
		},
	)
}

// fetchFeeds is a command that fetches feeds
func fetchFeeds(fm *feed.FeedManager) tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.LoadConfig()
		if err == nil && cfg != nil {
			fm.Feeds = cfg.Feeds
		}
		err2 := fm.RefreshFeeds()
		if err != nil {
			return fetchCompleteMsg{err: err}
		}
		return fetchCompleteMsg{err: err2}
	}
}

// Update handles updating the model based on messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// 配置视图中，将所有键盘输入传递给ConfigView处理
		if m.currentView == viewConfig {
			cv, cmd := m.configView.Handle(msg)
			m.configView = cv
			if cmd != nil {
				return m, cmd
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "r":
			// Refresh feeds
			m.loading = true
			m.statusMessage = "Refreshing feeds..."
			return m, fetchFeeds(m.feedManager)

		case "c":
			// 切换到配置视图
			if m.currentView != viewConfig {
				m.currentView = viewConfig
				// 更新配置视图中的feeds列表
				m.configView.UpdateFeeds(m.feedManager.Feeds)
			}

		case "tab", "h", "left":
			// Navigate between views
			if m.currentView == viewArticles {
				m.currentView = viewFeeds
			} else if m.currentView == viewArticleDetail {
				m.currentView = viewArticles
			} else if m.currentView == viewConfig {
				// 从配置视图返回到feeds视图
				m.currentView = viewFeeds
			}

		case "l", "right", "enter":
			// Navigate forward
			if m.currentView == viewFeeds && m.feedsList.Index() >= 0 {
				m.currentView = viewArticles
				m.currentFeed = m.feeds[m.feedsList.Index()]
				m.updateArticlesList()
			} else if m.currentView == viewArticles && m.articlesList.Index() >= 0 {
				m.currentView = viewArticleDetail
				if len(m.articlesList.Items()) > 0 {
					item := m.articlesList.SelectedItem().(Item)
					article := item.data.(feed.Article)
					m.articleView.SetArticle(article)
				}
			}

		case "j", "down":
			// Move down
			if m.currentView == viewFeeds {
				if m.feedsList.Index() < len(m.feedsList.Items())-1 {
					m.feedsList.Select(m.feedsList.Index() + 1)
					// Update articles list when feed selection changes
					if len(m.feedsList.Items()) > 0 {
						m.currentFeed = m.feeds[m.feedsList.Index()]
						m.updateArticlesList()
					}
				}
			} else if m.currentView == viewArticles {
				if m.articlesList.Index() < len(m.articlesList.Items())-1 {
					m.articlesList.Select(m.articlesList.Index() + 1)
				}
			} else if m.currentView == viewArticleDetail {
				m.articleView.ScrollDown()
			}

		case "k", "up":
			// Move up
			if m.currentView == viewFeeds {
				if m.feedsList.Index() > 0 {
					m.feedsList.Select(m.feedsList.Index() - 1)
					// Update articles list when feed selection changes
					if len(m.feedsList.Items()) > 0 {
						m.currentFeed = m.feeds[m.feedsList.Index()]
						m.updateArticlesList()
					}
				}
			} else if m.currentView == viewArticles {
				if m.articlesList.Index() > 0 {
					m.articlesList.Select(m.articlesList.Index() - 1)
				}
			} else if m.currentView == viewArticleDetail {
				m.articleView.ScrollUp()
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if !m.ready {
			// Initialize UI components on first resize
			m.feedsList = CreateFeedsList(m.feeds, 30, m.height)
			m.articlesList = list.New([]list.Item{}, ItemDelegate{}, m.width-34, m.height)
			m.articleView = NewArticleView(feed.Article{}, m.width-34, m.height)
			m.configView = NewConfigView(m.feedManager.Feeds, m.width-4, m.height)

			// Set list and view dimensions
			m.resizeComponents()
			m.ready = true

			// Start fetching feeds
			cmds = append(cmds, fetchFeeds(m.feedManager))
		} else {
			// Resize components
			m.resizeComponents()
		}

	case fetchStartMsg:
		m.loading = true
		m.statusMessage = "Loading feeds..."
		cmds = append(cmds, fetchFeeds(m.feedManager))

	case exitConfigMsg:
		m.currentView = viewFeeds
		return m, nil

	case saveConfigCompleteMsg:
		// 处理配置保存完成消息
		if msg.err != nil {
			m.errorMessage = fmt.Sprintf("Failed to save config: %v", msg.err)
		} else {
			// 配置保存成功后重新加载feed数据
			m.loading = true
			m.statusMessage = "Config saved, refreshing feeds..."
			return m, fetchFeeds(m.feedManager)
		}

	case fetchCompleteMsg:
		m.loading = false

		if msg.err != nil {
			m.errorMessage = fmt.Sprintf("Error: %v", msg.err)
			m.statusMessage = "Failed to load feeds"
		} else {
			m.errorMessage = ""
			m.statusMessage = "Feeds loaded successfully"

			// 用最新的 feedManager.Feeds 更新 m.feeds
			m.feeds = []string{"All"}
			for _, f := range m.feedManager.Feeds {
				m.feeds = append(m.feeds, f.Name)
			}
			m.feedsList = CreateFeedsList(m.feeds, 30, m.height-4)

			// Make sure we have a valid current feed selected
			if m.feedsList.Index() >= 0 && m.feedsList.Index() < len(m.feeds) {
				m.currentFeed = m.feeds[m.feedsList.Index()]
			}

			// Update the articles list
			m.updateArticlesList()

			// 更新配置视图中的feeds列表
			if m.configView != nil {
				m.configView.UpdateFeeds(m.feedManager.Feeds)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the UI
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// 根据当前view调整feedsList高亮
	if m.currentView == viewArticles {
		m.feedsList.Select(-1) // 取消高亮
	} else if m.currentView == viewFeeds && m.feedsList.Index() < 0 && len(m.feeds) > 0 {
		m.feedsList.Select(0)
	}
	feedsView := feedListStyle.Render(m.feedsList.View())

	var contentView string
	switch m.currentView {
	case viewFeeds:
		// 取消 articlesList 高亮
		m.articlesList.Select(-1)
		if m.loading {
			contentView = articleListStyle.Width(m.width - 34).Height(m.height - 2).Render("Loading...")
		} else {
			// Make sure articles list is updated for current feed
			if len(m.feedsList.Items()) > 0 && m.feedsList.Index() >= 0 {
				// Only update if not already showing the correct feed
				if m.currentFeed != m.feeds[m.feedsList.Index()] {
					m.currentFeed = m.feeds[m.feedsList.Index()]
					m.updateArticlesList()
				}
			}
			contentView = articleListStyle.Width(m.width - 34).Height(m.height - 4).Render(m.articlesList.View())
		}
	case viewArticles:
		if m.loading {
			contentView = articleListStyle.Width(m.width - 34).Height(m.height - 2).Render("Loading...")
		} else {
			contentView = articleListStyle.Width(m.width - 34).Height(m.height - 4).Render(m.articlesList.View())
		}
	case viewConfig:
		// 配置视图模式
		contentView = m.configView.Render()
		return contentView // 配置视图占据整个屏幕，不显示feeds列表
	default:
		contentView = m.articleView.Render()
	}

	// Status bar
	var statusBar string
	if m.loading {
		statusBar = statusBarStyle.Render("Loading...")
	} else if m.errorMessage != "" {
		statusBar = statusBarStyle.Copy().Foreground(lipgloss.Color("#FF0000")).Render(m.errorMessage)
	} else {
		help := []string{
			"j/k: navigate",
			"h/l: change view",
			"enter: select",
			"r: refresh",
			"c: config",
			"q: quit",
		}
		statusBar = statusBarStyle.Render(strings.Join(help, " • "))
	}

	// Layout
	mainContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		feedsView,
		contentView,
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		mainContent,
		statusBar,
	)
}

// resizeComponents updates the size of UI components
func (m *Model) resizeComponents() {
	m.feedsList.SetSize(30, m.height-4)
	m.articlesList.SetSize(m.width-34, m.height-4)
	m.articleView.SetSize(m.width-34, m.height-2)
	if m.configView != nil {
		m.configView.SetSize(m.width-4, m.height-4)
	}
}

// updateArticlesList filters and updates the articles list based on the selected feed
func (m *Model) updateArticlesList() {
	articles := m.feedManager.GetArticles()
	var filteredArticles []feed.Article

	if m.currentFeed == "All" {
		filteredArticles = articles
	} else {
		for _, a := range articles {
			if a.FeedName == m.currentFeed {
				filteredArticles = append(filteredArticles, a)
			}
		}
	}

	// 按时间排序文章（从新到旧）
	sort.Slice(filteredArticles, func(i, j int) bool {
		return filteredArticles[i].Published.After(filteredArticles[j].Published)
	})

	m.articlesList = CreateArticlesList(filteredArticles, m.width-34, m.height-4)
}
