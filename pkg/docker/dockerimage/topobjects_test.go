package dockerimage

import (
	"container/heap"
	"testing"
)

func TestTopObjects_List_NilSafety(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) TopObjects
		wantLen int
	}{
		{
			name: "nil TopObjects",
			setup: func(t *testing.T) TopObjects {
				return nil
			},
			wantLen: 0,
		},
		{
			name: "empty TopObjects",
			setup: func(t *testing.T) TopObjects {
				return NewTopObjects(0)
			},
			wantLen: 0,
		},
		{
			name: "TopObjects with nil elements",
			setup: func(t *testing.T) TopObjects {
				to := NewTopObjects(3)
				heap.Push(&to, &ObjectMetadata{Name: "file1", Size: 100})
				// Test both interface{} nil and typed nil
				heap.Push(&to, nil)
				heap.Push(&to, (*ObjectMetadata)(nil))
				heap.Push(&to, &ObjectMetadata{Name: "file2", Size: 200})
				return to
			},
			wantLen: 2, // Should only contain non-nil elements
		},
		{
			name: "TopObjects with multiple elements",
			setup: func(t *testing.T) TopObjects {
				to := NewTopObjects(3)
				heap.Push(&to, &ObjectMetadata{Name: "file1", Size: 100})
				heap.Push(&to, &ObjectMetadata{Name: "file2", Size: 200})
				heap.Push(&to, &ObjectMetadata{Name: "file3", Size: 50})
				return to
			},
			wantLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			to := tt.setup(t)
			result := to.List()

			if len(result) != tt.wantLen {
				t.Errorf("List() returned %d items, want %d", len(result), tt.wantLen)
			}

			// Verify the order is correct (descending by size)
			for i := 1; i < len(result); i++ {
				if result[i-1] == nil || result[i] == nil {
					t.Fatal("List() contains nil elements")
				}
				if result[i-1].Size < result[i].Size {
					t.Errorf("List() is not sorted in descending order: %d < %d at index %d", result[i-1].Size, result[i].Size, i)
				}
			}
		})
	}
}

func TestTopObjects_List_ModificationSafety(t *testing.T) {
	// Test that the original TopObjects is not modified by List()
	to := NewTopObjects(3)
	heap.Push(&to, &ObjectMetadata{Name: "file1", Size: 100})
	heap.Push(&to, &ObjectMetadata{Name: "file2", Size: 200})

	originalLen := to.Len()
	result := to.List()

	// Modify the result slice
	if len(result) > 0 {
		result[0] = &ObjectMetadata{Name: "modified", Size: 999}
	}

	// The original TopObjects should not be affected
	if to.Len() != originalLen {
		t.Errorf("Original TopObjects length changed from %d to %d", originalLen, to.Len())
	}
}
