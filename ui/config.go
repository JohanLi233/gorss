package ui

import (
	"fmt"
	"strings"

	"github.com/JohanLi233/gorss/config"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// 用于配置编辑的消息类型
type saveConfigMsg struct{ err error }

// ConfigView 表示配置界面
type ConfigView struct {
	feeds       []config.Feed
	width       int
	height      int
	cursor      int
	mode        string // "view", "add", "edit", "delete"
	activeInput int
	nameInput   textinput.Model
	urlInput    textinput.Model
	message     string
}

// NewConfigView 创建一个新的配置视图
func NewConfigView(feeds []config.Feed, width, height int) *ConfigView {
	nameInput := textinput.New()
	nameInput.Placeholder = "Feed 名称"
	nameInput.Focus()
	nameInput.CharLimit = 50
	nameInput.Width = 30

	urlInput := textinput.New()
	urlInput.Placeholder = "Feed URL"
	urlInput.CharLimit = 100
	urlInput.Width = 40

	return &ConfigView{
		feeds:       feeds,
		width:       width,
		height:      height,
		mode:        "view",
		nameInput:   nameInput,
		urlInput:    urlInput,
		activeInput: 0,
	}
}

// SetSize 更新视图的大小
func (cv *ConfigView) SetSize(width, height int) {
	cv.width = width
	cv.height = height
}

// Render 显示配置界面
func (cv *ConfigView) Render() string {
	switch cv.mode {
	case "view":
		return cv.renderViewMode()
	case "add", "edit":
		return cv.renderEditMode()
	case "delete":
		return cv.renderDeleteConfirmation()
	default:
		return cv.renderViewMode()
	}
}

// renderViewMode 显示feed列表
func (cv *ConfigView) renderViewMode() string {
	title := configTitleStyle.Render("RSS Feeds 配置")

	// 构建feeds列表
	var feedsContent strings.Builder

	if len(cv.feeds) == 0 {
		feedsContent.WriteString("没有配置的feeds。按 'a' 添加一个新feed。")
	} else {
		for i, feed := range cv.feeds {
			style := normalConfigItemStyle
			if i == cv.cursor {
				style = selectedConfigItemStyle
			}

			item := fmt.Sprintf("%s\n%s", feed.Name, feed.URL)
			feedsContent.WriteString(style.Render(item) + "\n\n")
		}
	}

	// 帮助信息
	help := []string{
		"↑/↓: 导航",
		"a: 添加feed",
		"e: 编辑feed",
		"d: 删除feed",
		"q: 返回主界面",
	}
	helpText := configHelpStyle.Render(strings.Join(help, " • "))

	// 消息提示
	var messageText string
	if cv.message != "" {
		messageText = configMessageStyle.Render(cv.message)
	}

	// 组合所有内容
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		configContentStyle.Width(cv.width-4).Render(feedsContent.String()),
		"",
		messageText,
		"",
		helpText,
	)

	return configViewStyle.Width(cv.width).Height(cv.height).Render(content)
}

// renderEditMode 显示添加/编辑feed的表单
func (cv *ConfigView) renderEditMode() string {
	var title string
	if cv.mode == "add" {
		title = configTitleStyle.Render("添加新Feed")
	} else {
		title = configTitleStyle.Render("编辑Feed")
	}

	// 表单字段
	nameField := fmt.Sprintf("名称: %s", cv.nameInput.View())
	urlField := fmt.Sprintf("URL:  %s", cv.urlInput.View())

	if cv.activeInput == 0 {
		nameField = selectedConfigFormStyle.Render(nameField)
		urlField = normalConfigFormStyle.Render(urlField)
	} else {
		nameField = normalConfigFormStyle.Render(nameField)
		urlField = selectedConfigFormStyle.Render(urlField)
	}

	// 帮助
	help := []string{
		"Tab: 切换字段",
		"Enter: 保存",
		"Esc: 取消",
	}
	helpText := configHelpStyle.Render(strings.Join(help, " • "))

	// 消息提示
	var messageText string
	if cv.message != "" {
		messageText = configMessageStyle.Render(cv.message)
	}

	// 组合所有内容
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		nameField,
		urlField,
		"",
		messageText,
		"",
		helpText,
	)

	return configViewStyle.Width(cv.width).Height(cv.height).Render(content)
}

// renderDeleteConfirmation 显示删除确认
func (cv *ConfigView) renderDeleteConfirmation() string {
	title := configTitleStyle.Render("删除Feed")

	// 确认信息
	confirmMessage := "确定要删除以下feed吗？"
	feedInfo := ""
	if cv.cursor >= 0 && cv.cursor < len(cv.feeds) {
		feed := cv.feeds[cv.cursor]
		feedInfo = fmt.Sprintf("%s (%s)", feed.Name, feed.URL)
	}

	confirmation := lipgloss.JoinVertical(
		lipgloss.Left,
		confirmMessage,
		"",
		selectedConfigItemStyle.Render(feedInfo),
		"",
		"按 'y' 确认删除，按 'n' 取消",
	)

	// 组合所有内容
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		confirmation,
	)

	return configViewStyle.Width(cv.width).Height(cv.height).Render(content)
}

// UpdateFeeds 更新feeds列表
func (cv *ConfigView) UpdateFeeds(feeds []config.Feed) {
	cv.feeds = feeds
	if cv.cursor >= len(cv.feeds) && len(cv.feeds) > 0 {
		cv.cursor = len(cv.feeds) - 1
	}
}

// Handle 处理键盘输入和其他事件
func (cv *ConfigView) Handle(msg tea.Msg) (*ConfigView, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch cv.mode {
		case "view":
			switch msg.String() {
			case "up", "k":
				if cv.cursor > 0 {
					cv.cursor--
				}
			case "down", "j":
				if cv.cursor < len(cv.feeds)-1 {
					cv.cursor++
				}
			case "a":
				// 切换到添加模式，直接丢弃本次'a'键输入
				cv.mode = "add"
				cv.nameInput.SetValue("")
				cv.urlInput.SetValue("")
				cv.nameInput.Focus()
				cv.urlInput.Blur()
				cv.activeInput = 0
				return cv, nil
			case "e":
				// 切换到编辑模式
				if len(cv.feeds) > 0 {
					cv.mode = "edit"
					feed := cv.feeds[cv.cursor]
					cv.nameInput.SetValue(feed.Name)
					cv.urlInput.SetValue(feed.URL)
					cv.nameInput.Focus()
					cv.urlInput.Blur()
					cv.activeInput = 0
				}
				return cv, nil
			case "d":
				// 切换到删除确认模式
				if len(cv.feeds) > 0 {
					cv.mode = "delete"
				}
			case "q", "h", "esc", "left":
				// 退出配置模式
				return cv, func() tea.Msg {
					return exitConfigMsg{}
				}
			}

		case "add", "edit":
			switch msg.String() {
			case "tab":
				// 切换输入字段
				if cv.activeInput == 0 {
					cv.activeInput = 1
					cv.nameInput.Blur()
					cv.urlInput.Focus()
				} else {
					cv.activeInput = 0
					cv.nameInput.Focus()
					cv.urlInput.Blur()
				}
			case "enter":
				// 保存变更
				name := strings.TrimSpace(cv.nameInput.Value())
				url := strings.TrimSpace(cv.urlInput.Value())

				if name == "" || url == "" {
					cv.message = "名称和URL不能为空！"
					return cv, nil
				}

				if cv.mode == "add" {
					// 添加新feed
					newFeed := config.Feed{
						Name: name,
						URL:  url,
					}
					cv.feeds = append(cv.feeds, newFeed)
					cv.cursor = len(cv.feeds) - 1
				} else {
					// 更新现有feed
					cv.feeds[cv.cursor] = config.Feed{
						Name: name,
						URL:  url,
					}
				}

				cv.mode = "view"
				cv.message = "Feed已保存。"

				// 返回保存配置的命令
				return cv, cv.saveConfig()

			case "esc":
				// 取消编辑
				cv.mode = "view"
				cv.message = ""
			}

		case "delete":
			switch msg.String() {
			case "y", "Y":
				// 确认删除
				if cv.cursor >= 0 && cv.cursor < len(cv.feeds) {
					// 删除feed
					cv.feeds = append(cv.feeds[:cv.cursor], cv.feeds[cv.cursor+1:]...)
					if len(cv.feeds) == 0 {
						cv.cursor = 0
					} else if cv.cursor >= len(cv.feeds) {
						cv.cursor = len(cv.feeds) - 1
					}
					cv.mode = "view"
					cv.message = "Feed已删除。"
					cv.nameInput.SetValue("")
					cv.urlInput.SetValue("")
					return cv, cv.saveConfig()
				}
			case "n", "N", "esc":
				// 取消删除
				cv.mode = "view"
				cv.message = ""
			}
		}

	case saveConfigMsg:
		if msg.err != nil {
			cv.message = fmt.Sprintf("保存失败: %v", msg.err)
		} else {
			cv.message = "配置已保存。"
		}
	}

	// 处理输入字段更新
	if cv.mode == "add" || cv.mode == "edit" {
		var cmd tea.Cmd
		if cv.activeInput == 0 {
			cv.nameInput, cmd = cv.nameInput.Update(msg)
			cmds = append(cmds, cmd)
		} else {
			cv.urlInput, cmd = cv.urlInput.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return cv, tea.Batch(cmds...)
}

// saveConfig 保存配置到文件
func (cv *ConfigView) saveConfig() tea.Cmd {
	// 调用保存配置函数，传入当前feeds列表
	return saveConfig(cv.feeds)
}
