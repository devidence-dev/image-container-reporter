package types

import "testing"

func TestScanResult_HasUpdates(t *testing.T) {
	tests := []struct {
		name     string
		result   ScanResult
		expected bool
	}{
		{
			name: "has updates",
			result: ScanResult{
				UpdatesAvailable: []ImageUpdate{
					{ServiceName: "nginx"},
				},
			},
			expected: true,
		},
		{
			name: "no updates",
			result: ScanResult{
				UpdatesAvailable: []ImageUpdate{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.result.HasUpdates()
			if result != tt.expected {
				t.Errorf("ScanResult.HasUpdates() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestScanResult_Summary(t *testing.T) {
	tests := []struct {
		name     string
		result   ScanResult
		expected string
	}{
		{
			name: "with updates",
			result: ScanResult{
				UpdatesAvailable: []ImageUpdate{
					{ServiceName: "nginx"},
				},
				UpToDateServices: []string{"redis", "postgres"},
			},
			expected: "1 updates available, 2 services up to date",
		},
		{
			name: "no updates",
			result: ScanResult{
				UpdatesAvailable: []ImageUpdate{},
				UpToDateServices: []string{"nginx", "redis"},
			},
			expected: "All 2 services are up to date",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.result.Summary()
			if result != tt.expected {
				t.Errorf("ScanResult.Summary() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestImageUpdate_IsSignificant(t *testing.T) {
	tests := []struct {
		name     string
		update   ImageUpdate
		expected bool
	}{
		{
			name: "major update",
			update: ImageUpdate{
				UpdateType: UpdateTypeMajor,
			},
			expected: true,
		},
		{
			name: "minor update",
			update: ImageUpdate{
				UpdateType: UpdateTypeMinor,
			},
			expected: true,
		},
		{
			name: "patch update",
			update: ImageUpdate{
				UpdateType: UpdateTypePatch,
			},
			expected: false,
		},
		{
			name: "unknown update",
			update: ImageUpdate{
				UpdateType: UpdateTypeUnknown,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.update.IsSignificant()
			if result != tt.expected {
				t.Errorf("ImageUpdate.IsSignificant() = %v, want %v", result, tt.expected)
			}
		})
	}
}