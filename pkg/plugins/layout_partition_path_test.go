package plugins

import (
	"testing"
)

func TestPartitionDevicePath(t *testing.T) {
	tests := []struct {
		name     string
		device   string
		partNum  int
		expected string
	}{
		{
			name:     "sda disk",
			device:   "/dev/sda",
			partNum:  1,
			expected: "/dev/sda1",
		},
		{
			name:     "vda disk",
			device:   "/dev/vda",
			partNum:  2,
			expected: "/dev/vda2",
		},
		{
			name:     "mmcblk device requires p separator",
			device:   "/dev/mmcblk0",
			partNum:  4,
			expected: "/dev/mmcblk0p4",
		},
		{
			name:     "nvme device requires p separator",
			device:   "/dev/nvme0n1",
			partNum:  1,
			expected: "/dev/nvme0n1p1",
		},
		{
			name:     "loop device requires p separator",
			device:   "/dev/loop0",
			partNum:  1,
			expected: "/dev/loop0p1",
		},
		{
			name:     "xvda disk no separator",
			device:   "/dev/xvda",
			partNum:  3,
			expected: "/dev/xvda3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := partitionDevicePath(tt.device, tt.partNum)
			if got != tt.expected {
				t.Errorf("partitionDevicePath(%q, %d) = %q, want %q", tt.device, tt.partNum, got, tt.expected)
			}
		})
	}
}
