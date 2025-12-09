// Copyright (c) 2025 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package cliui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var quitTextStyle = lipgloss.NewStyle().Margin(1, 0, 2, 4)

type model struct {
	list     list.Model
	index    int
	choice   string
	quitting bool
}

func (m *model) Init() tea.Cmd {
	return nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEscape:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEnter:
			i, ok := m.list.SelectedItem().(item)
			if ok {
				m.index = m.list.Index()
				m.choice = string(i)
			}
			return m, tea.Quit
		}

		// workaround to ensure 'q' has consistent behaviour
		// as 'ctrl+c' and 'ESC'.
		if msg.String() == "q" {
			m.quitting = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *model) View() string {
	if m.choice != "" {
		return quitTextStyle.Render(fmt.Sprintf("Selected %d: %s", m.index, m.choice))
	}
	if m.quitting {
		return quitTextStyle.Render("Selection cancelled.")
	}

	return "\n" + m.list.View()
}
