package assertions

import (
	"testing"
	"smith/internal/source/model"
	"github.com/stretchr/testify/assert"
)

type mockTB struct {
	*testing.T
	fatalfCalled bool
	fatalfFormat string
	fatalfArgs   []interface{}
}

func (m *mockTB) Fatalf(format string, args ...interface{}) {
	m.fatalfCalled = true
	m.fatalfFormat = format
	m.fatalfArgs = args
}

func TestRequireLoopState(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mt := &mockTB{T: t}
		RequireLoopState(mt, model.LoopStateRunning, model.LoopStateRunning)
		assert.False(t, mt.fatalfCalled)
	})

	t.Run("failure", func(t *testing.T) {
		mt := &mockTB{T: t}
		RequireLoopState(mt, model.LoopStateUnresolved, model.LoopStateRunning)
		assert.True(t, mt.fatalfCalled)
		expectedFormat := "unexpected loop state: got=%s want=%s"
		assert.Equal(t, expectedFormat, mt.fatalfFormat)
		assert.Equal(t, []interface{}{model.LoopStateUnresolved, model.LoopStateRunning}, mt.fatalfArgs)
	})
}

func TestRequireNonEmpty(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mt := &mockTB{T: t}
		RequireNonEmpty(mt, "field1", "value1")
		assert.False(t, mt.fatalfCalled)
	})

	t.Run("failure empty", func(t *testing.T) {
		mt := &mockTB{T: t}
		RequireNonEmpty(mt, "field1", "")
		assert.True(t, mt.fatalfCalled)
		assert.Equal(t, "expected non-empty value for %s", mt.fatalfFormat)
		assert.Equal(t, []interface{}{"field1"}, mt.fatalfArgs)
	})

	t.Run("failure whitespace", func(t *testing.T) {
		mt := &mockTB{T: t}
		RequireNonEmpty(mt, "field1", "   \t\n  ")
		assert.True(t, mt.fatalfCalled)
	})
}

func TestRequireJournalMessage(t *testing.T) {
	entries := []model.JournalEntry{
		{Message: "starting loop processing"},
		{Message: "fetching data from upstream"},
		{Message: "completed successfully"},
	}

	t.Run("success", func(t *testing.T) {
		mt := &mockTB{T: t}
		RequireJournalMessage(mt, entries, "fetching data")
		assert.False(t, mt.fatalfCalled)
	})

	t.Run("success exact match", func(t *testing.T) {
		mt := &mockTB{T: t}
		RequireJournalMessage(mt, entries, "starting loop processing")
		assert.False(t, mt.fatalfCalled)
	})

	t.Run("failure not found", func(t *testing.T) {
		mt := &mockTB{T: t}
		RequireJournalMessage(mt, entries, "error occurred")
		assert.True(t, mt.fatalfCalled)
		assert.Equal(t, "expected journal message containing %q", mt.fatalfFormat)
		assert.Equal(t, []interface{}{"error occurred"}, mt.fatalfArgs)
	})

	t.Run("failure empty entries", func(t *testing.T) {
		mt := &mockTB{T: t}
		RequireJournalMessage(mt, nil, "anything")
		assert.True(t, mt.fatalfCalled)
	})
}
