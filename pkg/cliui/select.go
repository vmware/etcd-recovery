// Copyright (c) 2025 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package cliui

import (
	"errors"
	"log"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	defaultWidth = 20
	listHeight   = 14
)

var (
	titleStyle      = lipgloss.NewStyle().MarginLeft(2)
	paginationStyle = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle       = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
)

// Select displays an interactive command-line menu with a given title
// and a list of options, allowing the user to choose one of them.
//
// Parameters:
//   - title: A string that will be displayed as the heading for the menu.
//   - options: A slice of strings representing the selectable options in the menu.
//
// Returns:
//   - int: The zero-based index of the option selected by the user in the `options` slice.
//   - string: The actual string value of the option selected by the user.
//   - error: An error if the selection process fails or is canceled by the user.
//
// Example usage:
//
//	options := []string{"etcd-vm1", "etcd-vm2", "etcd-vm3"}
//	idx, choice, err := cliui.Select("Please choose one VM:", options)
//	if err != nil {
//	    fmt.Println("Selection canceled or failed:", err)
//	    return
//	}
//	fmt.Printf("You selected option %d: %s\n", idx, choice)
func Select(title string, options []string) (int, string, error) {
	var items []list.Item
	for _, option := range options {
		items = append(items, item(option))
	}

	if len(items) == 0 {
		return -1, "", errors.New("no options provided")
	}

	l := list.New(items, itemDelegate{}, defaultWidth, listHeight)
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle

	m := &model{
		list:  l,
		index: -1,
	}

	if _, err := tea.NewProgram(m).Run(); err != nil {
		log.Fatalf("error selecting from CLI menu: %v", err)
	}

	if m.quitting {
		return -1, "", errors.New("user cancelled")
	}

	return m.index, m.choice, nil
}
