package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfirmModal_RendersWithTitleAndBody(t *testing.T) {
	m := defaultModel()
	m.width = 80
	m.height = 24
	m.confirmOpen = true
	m.confirmTitle = "Stow git?"
	m.confirmBody = "3 files to link\n1 already linked"
	m.confirmHint = "enter confirm · esc cancel"

	result := confirmModal(m)

	assert.Contains(t, result, "Stow git?")
	assert.Contains(t, result, "3 files to link")
}

func TestConfirmModal_NotRenderedWhenClosed(t *testing.T) {
	m := defaultModel()
	m.width = 80
	m.height = 24
	m.confirmOpen = false

	result := confirmModal(m)

	assert.Empty(t, result)
}

func TestConfirmModal_DangerStyle(t *testing.T) {
	m := defaultModel()
	m.width = 80
	m.height = 24
	m.confirmOpen = true
	m.confirmTitle = "Delete git?"
	m.confirmDanger = true
	m.confirmBody = "Permanently removes 5 files."
	m.confirmHint = "enter confirm · esc cancel"

	result := confirmModal(m)

	assert.Contains(t, result, "Delete git?")
	assert.Contains(t, result, "Permanently removes 5 files.")
}

func defaultModel() Model {
	return NewModel(nil, "test")
}
