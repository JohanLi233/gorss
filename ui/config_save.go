package ui

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lizhonghan/gorss/config"
	"github.com/spf13/viper"
)

// ConfigToSave 表示要保存的配置结构，与config包中的Config对应
type ConfigToSave struct {
	Feeds []config.Feed `mapstructure:"feeds"`
}

// saveConfig 保存配置更改到文件
func saveConfig(feeds []config.Feed) tea.Cmd {
	return func() tea.Msg {
		// 获取配置文件路径
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return saveConfigCompleteMsg{err: fmt.Errorf("无法获取用户主目录: %w", err)}
		}

		configDir := filepath.Join(homeDir, ".config", "gorss")
		configPath := filepath.Join(configDir, "config.yaml")

		// 确保配置目录存在
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return saveConfigCompleteMsg{err: fmt.Errorf("无法创建配置目录: %w", err)}
		}

		// 使用viper保存配置
		v := viper.New()
		v.SetConfigFile(configPath)
		v.SetConfigType("yaml")

		// 设置配置内容
		v.Set("feeds", feeds)

		// 保存到文件
		if err := v.WriteConfig(); err != nil {
			// 如果文件不存在，尝试创建
			if _, ok := err.(viper.ConfigFileNotFoundError); ok {
				if err := v.SafeWriteConfig(); err != nil {
					return saveConfigCompleteMsg{err: fmt.Errorf("无法创建配置文件: %w", err)}
				}
			} else {
				return saveConfigCompleteMsg{err: fmt.Errorf("无法保存配置文件: %w", err)}
			}
		}

		return saveConfigCompleteMsg{err: nil}
	}
}
