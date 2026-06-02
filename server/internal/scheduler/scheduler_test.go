package scheduler

import "testing"

func TestComputeAgentDomains(t *testing.T) {
	result := ComputeAgentDomains([]uint{1, 2, 3}, []uint{4, 5}, []uint{2})
	expected := []uint{1, 3, 4, 5}
	if !equal(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestComputeAgentDomains_NoExcludes(t *testing.T) {
	result := ComputeAgentDomains([]uint{1, 2}, []uint{3}, []uint{})
	expected := []uint{1, 2, 3}
	if !equal(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestComputeAgentDomains_ExcludeNonGlobal(t *testing.T) {
	result := ComputeAgentDomains([]uint{1}, []uint{}, []uint{99})
	expected := []uint{1}
	if !equal(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func equal(a, b []uint) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
