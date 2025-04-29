package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// AskLLMView is a view for entering a prompt to ask the LLM
// It is a simple wrapper around a text input

type AskLLMView struct {
	input        textinput.Model
	width        int
	height       int
	message      string   // For error or status
	submitting   bool     // 是否正在提交
	scrollOffset int      // 回复内容滚动偏移
	contentLines []string // 处理后的内容行（用于分页）
}

func NewAskLLMView(width, height int) *AskLLMView {
	ti := textinput.New()
	ti.Placeholder = "ASK LLM..."
	ti.Focus()
	ti.Width = width - 8
	return &AskLLMView{
		input:      ti,
		width:      width,
		height:     height,
		message:    "",
		submitting: false,
	}
}

func (v *AskLLMView) Update(msg tea.Msg) (*AskLLMView, bool, string) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		// 正在提交中，只处理ESC键
		if v.submitting {
			if m.Type == tea.KeyEsc {
				return v, true, ""
			}
			return v, false, ""
		}

		// 如果有结果显示中，处理翻页操作
		if len(v.contentLines) > 0 {
			// 使用字符串匹配按键
			switch msg := m.String(); {
			case msg == "esc":
				// 重置状态，确保下次进入时是全新状态
				v.scrollOffset = 0
				v.contentLines = nil
				return v, true, ""
			case msg == "down" || msg == " " || msg == "j":
				// 向下翻页
				v.scrollDown()
				return v, false, ""
			case msg == "up" || msg == "k" || msg == "backspace":
				// 向上翻页
				v.scrollUp()
				return v, false, ""
			}
			return v, false, ""
		}

		// 处理输入提交
		switch m.Type {
		case tea.KeyEnter:
			prompt := v.input.Value()
			v.input.SetValue("") // 清空输入框
			v.submitting = true
			v.message = "正在向 LLM 提问，请稍候..."
			return v, true, prompt
		case tea.KeyEsc:
			return v, true, ""
		}
	}

	// 处理文本输入
	var cmd tea.Cmd
	v.input, cmd = v.input.Update(msg)
	_ = cmd // unused
	return v, false, ""
}

// 向上滚动
func (v *AskLLMView) scrollUp() {
	if v.scrollOffset > 0 {
		v.scrollOffset--
	}
}

// 向下滚动
func (v *AskLLMView) scrollDown() {
	// 计算可见行数
	contentHeight := v.height - 10 // 减去标题、问题、提示等行
	if v.scrollOffset < len(v.contentLines)-contentHeight {
		v.scrollOffset++
	}
}

func (v *AskLLMView) View(result ...string) string {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#4B7BEC")).
		Padding(1, 2).
		Width(v.width).
		Height(v.height)

	// 检查是否有回复内容
	hasResult := len(result) > 0 && result[0] != ""

	// 回复模式：显示提问和回复内容
	if hasResult {
		// 创建标题样式
		promptStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#2E86DE"))

		answerTitleStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#10AC84"))

		// 获取问题内容
		question := v.input.Value()
		if question == "" {
			question = "请输入问题"
		}

		// 分割并包装内容为行
		allLines := strings.Split(result[0], "\n")
		contentWidth := v.width - 8 // 考虑padding和border
		var wrappedLines []string
		for _, line := range allLines {
			for len(line) > contentWidth {
				wrappedLines = append(wrappedLines, line[:contentWidth])
				line = line[contentWidth:]
			}
			wrappedLines = append(wrappedLines, line)
		}
		v.contentLines = wrappedLines
		// 校正滚动偏移
		contentHeight := v.height - 10 // 减去标题、问题、提示等行
		if contentHeight < 3 {
			contentHeight = 3
		}
		maxOffset := len(v.contentLines) - contentHeight
		if maxOffset < 0 {
			maxOffset = 0
		}
		if v.scrollOffset < 0 {
			v.scrollOffset = 0
		}
		if v.scrollOffset > maxOffset {
			v.scrollOffset = maxOffset
		}

		// 计算可视区域
		if contentHeight < 3 {
			contentHeight = 3 // 最小可视区域
		}

		// 截取当前可见的内容行，添加安全检查
		start := v.scrollOffset
		// 防止起始偏移超过数组长度
		if start >= len(v.contentLines) {
			start = 0
			v.scrollOffset = 0 // 重置偏移
		}

		end := start + contentHeight
		if end > len(v.contentLines) {
			end = len(v.contentLines)
		}

		// 确保 start < end
		if start >= end {
			// 设置安全的默认值
			if len(v.contentLines) > 0 {
				start = 0
				end = min(contentHeight, len(v.contentLines))
				v.scrollOffset = 0 // 重置偏移
			} else {
				// 内容为空时的处理
				return box.Render("LLM 回复为空...\n\n按 ESC 返回")
			}
		}

		var visibleContent string
		if len(v.contentLines) > 0 {
			visibleContent = strings.Join(v.contentLines[start:end], "\n")
		} else {
			visibleContent = ""
		}

		// 添加分页指示器
		pageInfo := fmt.Sprintf("--- 页 %d/%d ---", v.scrollOffset+1,
			max(1, len(v.contentLines)-contentHeight+1))

		// 添加翻页提示
		pageControls := "[↑/k: 上翻] [↓/j/空格: 下翻] [Esc: 返回]"

		// 构建显示内容
		content := lipgloss.JoinVertical(
			lipgloss.Left,
			promptStyle.Render("问题:"),
			question,
			"",
			answerTitleStyle.Render("LLM 回复:"),
			visibleContent,
			"",
			pageInfo,
			pageControls,
		)

		return box.Render(content)
	}

	// 输入模式：显示提示和输入框
	desc := "Ask LLM (按 Enter 提交, Esc 取消):\n"
	if v.submitting {
		return box.Render("正在请求 LLM 回复...\n\n请稍等，这可能需要一些时间")
	}

	return box.Render(
		desc + v.input.View() + "\n" + v.message,
	)
}

// max 返回两个整数中的较大值
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
