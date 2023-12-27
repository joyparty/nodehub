package cluster

import (
	"testing"
)

func TestAddrToint64(t *testing.T) {
	// Test cases
	testCases := []struct {
		remoteAddr string
		expected   int64
	}{
		{
			remoteAddr: "127.0.0.1:8080",
			expected:   2130706433,
		},
		{
			remoteAddr: "192.168.0.1:1234",
			expected:   3232235521,
		},
		{
			remoteAddr: "[::1]:8080",
			expected:   1,
		},
		{
			remoteAddr: "[2001:0db8:85a3:0000:0000:8a2e:0370:7334]:1234",
			expected:   151930230829876,
		},
	}

	// Run the test cases
	for _, tc := range testCases {
		result, err := addrToint64(tc.remoteAddr)
		if result != tc.expected {
			t.Errorf("addrToint64(%s) = %d, expected %d, err %v", tc.remoteAddr, result, tc.expected, err)
		}
	}
}
