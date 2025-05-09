package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/JohanLi233/gorss/config"
	"github.com/JohanLi233/gorss/feed"
	"github.com/JohanLi233/gorss/llm"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// View states
const (
	viewFeeds = iota
	viewArticles
	viewArticleDetail
	viewConfig  // 配置视图
	viewSummary // 摘要视图
	viewAskLLM  // Ask LLM prompt view
)

// Messages
type fetchCompleteMsg struct{ err error }
type fetchStartMsg struct{}
type saveConfigCompleteMsg struct{ err error }
type exitConfigMsg struct{}

// Model represents the main application UI model
type Model struct {
	feedManager  *feed.FeedManager
	feeds        []string
	feedsList    list.Model
	articlesList list.Model
	articleView  *ArticleView
	configView   *ConfigView // 配置视图
	askLLMView   *AskLLMView // Ask LLM prompt view
	currentView  int
	currentFeed  string
	width        int
	height       int
	ready        bool
	loading      bool

	errorMessage   string
	statusMessage  string
	ollamaConfig   llm.OllamaConfig
	askLLMResult   string                        // last ask result
	liveResponseCh chan llm.StreamingResponseMsg // channel for streaming responses

	// 多选模式及选中索引
	multiSelectMode        bool
	selectedArticleIndexes map[int]struct{}
}

type askLLMCompleteMsg struct {
	prompt string
	result string
	err    error
}

type streamingLLMResponseMsg struct {
	content string
	done    bool
	err     error
}

// NewModel creates a new application model
func NewModel(feedManager *feed.FeedManager) Model {
	// Add 'All' feed option at the beginning
	feeds := []string{"All"}
	for _, f := range feedManager.Feeds {
		feeds = append(feeds, f.Name)
	}

	// Initialize with default Ollama config
	ollamaConfig := llm.DefaultOllamaConfig()

	// Create the model
	m := Model{
		feedManager:  feedManager,
		feeds:        feeds,
		currentView:  viewFeeds,
		currentFeed:  "All", // Start with 'All' selected
		loading:      false, // Start with loading false since we're using cached data
		ollamaConfig: ollamaConfig,
	}

	// Initialize the feeds list
	m.feedsList = CreateFeedsList(feeds, 30, 20) // Height will be adjusted later

	// Initialize articles list with cached articles
	m.updateArticlesList()

	// Initialize the article view with empty article initially
	m.articleView = NewArticleView(feed.Article{}, 80, 20) // Size will be adjusted later

	// Initialize the config view with current feeds
	m.configView = NewConfigView(feedManager.Feeds, 80, 20) // Size will be adjusted later

	// Initialize the Ask LLM view
	m.askLLMView = NewAskLLMView(80, 8)

	return m
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		func() tea.Msg {
			// Just load configuration without refreshing feeds
			cfg, err := config.LoadConfig()
			if err == nil && cfg != nil {
				m.feedManager.Feeds = cfg.Feeds
			}
			// Instead of fetching, immediately initialize the UI with cached articles
			return fetchCompleteMsg{err: nil}
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
		if m.currentView == viewAskLLM {
			newView, submitted, prompt := m.askLLMView.Update(msg)
			m.askLLMView = newView
			if submitted {
				if prompt == "" {
					// 取消 (Esc)
					// 清空历史记录，确保下次进入时没有之前的对话
					m.askLLMResult = ""
					m.currentView = viewFeeds
					return m, nil
				}
				// Call LLM with prompt using streaming API
				m.askLLMView.submitting = true
				m.statusMessage = "Asking LLM..."

				// 重置历史内容，并重置分页状态
				m.askLLMResult = ""
				if m.askLLMView != nil {
					m.askLLMView.scrollOffset = 0
					m.askLLMView.contentLines = nil
				}


				// 清理之前的响应通道（如果有）
				if m.liveResponseCh != nil {
					// 尝试从通道中读取剩余内容以确保goroutine不会阻塞
					go func(ch chan llm.StreamingResponseMsg) {
						for range ch {
							// 忽略所有剩余内容
						}
					}(m.liveResponseCh)
				}

				// 创建新的响应通道并存储在模型中
				m.liveResponseCh = llm.AskOllamaStreaming(prompt, m.ollamaConfig)

				// 创建命令来获取第一个响应块
				cmd := func() tea.Msg {
					// 从通道读取第一个响应
					if m.liveResponseCh == nil {
						return streamingLLMResponseMsg{content: "", done: true, err: fmt.Errorf("response channel was closed")}
					}

					chunk, ok := <-m.liveResponseCh
					if !ok {
						// 通道已关闭
						return streamingLLMResponseMsg{content: "", done: true, err: nil}
					}

					if chunk.Err != nil {
						// 响应出错
						return streamingLLMResponseMsg{content: "", done: true, err: chunk.Err}
					}

					return streamingLLMResponseMsg{content: chunk.Content, done: chunk.Done, err: nil}
				}
				return m, cmd
			}
			return m, nil
		}

		// 配置视图中，将所有键盘输入传递给ConfigView处理
		if m.currentView == viewConfig {
			cv, cmd := m.configView.Handle(msg)
			m.configView = cv
			if cmd != nil {
				return m, cmd
			}
			return m, nil
		}

		// Ask LLM entry: 'a' from feeds, articles, or article detail view
		if (m.currentView == viewFeeds || m.currentView == viewArticles || m.currentView == viewArticleDetail) && msg.String() == "a" {
			presetPrompt := ""
			
			// 多选模式下处理
			if m.currentView == viewArticles && m.multiSelectMode && len(m.selectedArticleIndexes) > 0 {
				// 拼接所有选中文章内容
				var prompt strings.Builder
				for idx := range m.selectedArticleIndexes {
					item := m.articlesList.Items()[idx].(Item)
					article, ok := item.data.(feed.Article)
					if ok {
						prompt.WriteString("标题: ")
						prompt.WriteString(article.Title)
						prompt.WriteString("\n内容: ")
						prompt.WriteString(article.Content)
						prompt.WriteString("\n\n")
					}
				}
				presetPrompt = prompt.String()
				// 退出多选模式
				m.multiSelectMode = false
				m.selectedArticleIndexes = nil
			} else if m.currentView == viewArticleDetail && m.articleView != nil {
				// 单篇文章详情页处理
				article := m.articleView.article
				if article.Title != "" || article.Content != "" {
					presetPrompt = fmt.Sprintf("Title: %s\nContent: %s\n\n", article.Title, article.Content)
				}
			}
			
			// 创建新的 AskLLMView 并填充预设内容
			m.askLLMView = NewAskLLMView(m.width, m.height)
			if presetPrompt != "" {
				m.askLLMView.input.SetValue(presetPrompt)
			}
			m.currentView = viewAskLLM
			return m, nil
		}

		// 摘要视图中，处理滚动（仅用于 LLM 结果浏览）
		if m.currentView == viewSummary {
			switch msg.String() {
			case "j", "down":

				return m, nil
			case "k", "up":

				return m, nil
			case "tab", "h", "left", "esc":
				// Return to previous view
				m.currentView = viewArticles
				return m, nil
			}
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
				// 多选模式下回车处理
				if m.multiSelectMode && len(m.selectedArticleIndexes) > 0 {
					// 拼接所有选中文章内容
					var prompt strings.Builder
					for idx := range m.selectedArticleIndexes {
						item := m.articlesList.Items()[idx].(Item)
						article, ok := item.data.(feed.Article)
						if ok {
							prompt.WriteString("标题: ")
							prompt.WriteString(article.Title)
							prompt.WriteString("\n内容: ")
							prompt.WriteString(article.Content)
							prompt.WriteString("\n\n")
						}
					}
					// 跳转 askLLMView 并填充 prompt
					m.currentView = viewAskLLM
					m.askLLMView.input.SetValue(prompt.String())
					m.multiSelectMode = false
					m.selectedArticleIndexes = nil
					m.statusMessage = ""
				} else {
					// 正常模式进入文章详情
					m.currentView = viewArticleDetail
					if len(m.articlesList.Items()) > 0 {
						item := m.articlesList.SelectedItem().(Item)
						article := item.data.(feed.Article)
						m.articleView.SetArticle(article)
					}
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
				if m.multiSelectMode {
					// 多选模式下，j/k 只移动高亮
					if m.articlesList.Index() < len(m.articlesList.Items())-1 {
						m.articlesList.Select(m.articlesList.Index() + 1)
					}
				} else {
					if m.articlesList.Index() < len(m.articlesList.Items())-1 {
						m.articlesList.Select(m.articlesList.Index() + 1)
					}
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

		case "v":
			// 多选模式切换
			if m.currentView == viewArticles {
				if !m.multiSelectMode {
					// 开启多选模式
					m.multiSelectMode = true
					m.selectedArticleIndexes = make(map[int]struct{})
					m.statusMessage = "多选模式：空格选择，回车发送，v退出"
				} else {
					// 关闭多选模式
					m.multiSelectMode = false
					m.selectedArticleIndexes = nil
					m.statusMessage = ""
				}
				return m, nil
			}

		case " ":
			// 多选模式下的选择/取消
			if m.currentView == viewArticles && m.multiSelectMode {
				idx := m.articlesList.Index()
				if _, ok := m.selectedArticleIndexes[idx]; ok {
					// 取消选择
					delete(m.selectedArticleIndexes, idx)
				} else {
					// 添加选择
					m.selectedArticleIndexes[idx] = struct{}{}
				}
				return m, nil
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

			// 不自动加载远程 feeds，避免网络错误
			// cmds = append(cmds, fetchFeeds(m.feedManager))
			m.statusMessage = "启动完成，按 r 键刷新 RSS"
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
			m.statusMessage = "Loaded from cache"

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

	case streamingLLMResponseMsg:
		// enable paging during stream
		if m.askLLMView != nil {
			m.askLLMView.submitting = false
		}

		if msg.err != nil {
			// 处理流式响应错误
			m.errorMessage = "LLM error: " + msg.err.Error()
			m.statusMessage = "Ask LLM failed"

			if m.askLLMView != nil {
				m.askLLMView.submitting = false
			}
			return m, nil
		}

		// 总是拼接收到的内容（包括最后一块）
		if msg.content != "" {
			m.askLLMResult += msg.content
		}

		if !msg.done {
			// 创建命令来读取下一个响应块
			return m, func() tea.Msg {
				// 从同一个通道读取下一个响应
				if m.liveResponseCh == nil {
					return streamingLLMResponseMsg{content: "", done: true, err: fmt.Errorf("response channel was closed")}
				}

				chunk, ok := <-m.liveResponseCh
				if !ok {
					// 通道已关闭
					return streamingLLMResponseMsg{content: "", done: true, err: nil}
				}

				if chunk.Err != nil {
					// 响应出错
					return streamingLLMResponseMsg{content: "", done: true, err: chunk.Err}
				}

				return streamingLLMResponseMsg{content: chunk.Content, done: chunk.Done, err: nil}
			}
		} else {
			// 流式传输完成
			m.statusMessage = "LLM answered"
			if m.askLLMView != nil {
				m.askLLMView.submitting = false
			}
		}

	case askLLMCompleteMsg:
		// 处理非流式响应或流式响应的错误情况
		if m.askLLMView != nil {
			m.askLLMView.submitting = false
		}

		if msg.err != nil {
			m.errorMessage = "LLM error: " + msg.err.Error()
			m.statusMessage = "Ask LLM failed"
			m.askLLMResult = ""
		} else {
			m.errorMessage = ""
			m.statusMessage = "LLM answered"
			m.askLLMResult = msg.result
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the UI
func (m Model) View() string {
	// 多选高亮同步到 list.go 全局变量
	if m.currentView == viewArticles && m.multiSelectMode {
		MultiSelectMode = true
		SelectedArticleIndexes = m.selectedArticleIndexes
	} else {
		MultiSelectMode = false
		SelectedArticleIndexes = nil
	}

	if !m.ready {
		return "Initializing..."
	}

	feedsView := feedListStyle.Render(m.feedsList.View())

	var contentView string
	switch m.currentView {
	case viewAskLLM:
		return m.askLLMView.View(m.askLLMResult)
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
	} else if m.currentView == viewArticles && m.multiSelectMode {
		// 多选模式下显示特殊状态栏
		selectedCount := len(m.selectedArticleIndexes)
		statusText := fmt.Sprintf("多选模式 | 已选择: %d | 空格: 选择/取消 | 回车/a: 发送至LLM | v: 退出", selectedCount)
		statusBar = statusBarStyle.Copy().Foreground(bubbleTeaColor).Render(statusText)
	} else {
		help := []string{
			"j/k: navigate",
			"h/l: change view",
			"enter: select",
			"r: refresh",
			"a: ask LLM",
			"c: config",
			"q: quit",
		}

		// 在 articles 页面显示 v 多选提示
		if m.currentView == viewArticles {
			help = append(help, "v: multi-select")
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
	if m.askLLMView != nil {
		m.askLLMView.width = m.width
		m.askLLMView.height = m.height
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
