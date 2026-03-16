package ecsexecpf

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePortMappings_SingleMapping(t *testing.T) {
	result, err := ParsePortMappings("8080:80")
	require.NoError(t, err)
	assert.Equal(t, []PortMapping{{LocalPort: 8080, RemotePort: 80}}, result)
}

func TestParsePortMappings_MultipleMappings(t *testing.T) {
	result, err := ParsePortMappings("8080:80, 8443:443")
	require.NoError(t, err)
	expected := []PortMapping{
		{LocalPort: 8080, RemotePort: 80},
		{LocalPort: 8443, RemotePort: 443},
	}
	assert.Equal(t, expected, result)
}

func TestParsePortMappings_EmptyInput(t *testing.T) {
	_, err := ParsePortMappings("")
	require.ErrorContains(t, err, "cannot be empty")
}

func TestParsePortMappings_InvalidFormat(t *testing.T) {
	_, err := ParsePortMappings("8080")
	require.ErrorContains(t, err, "expected local:remote")
}

func TestParsePortMappings_InvalidLocalPort(t *testing.T) {
	_, err := ParsePortMappings("abc:80")
	require.ErrorContains(t, err, "invalid local port")
}

func TestParsePortMappings_InvalidRemotePort(t *testing.T) {
	_, err := ParsePortMappings("8080:abc")
	require.ErrorContains(t, err, "invalid remote port")
}

func TestParsePortMappings_PortOutOfRange(t *testing.T) {
	_, err := ParsePortMappings("70000:80")
	require.ErrorContains(t, err, "must be between")

	_, err = ParsePortMappings("8080:70000")
	require.ErrorContains(t, err, "must be between")
}

func TestParsePortMappings_TrimsWhitespace(t *testing.T) {
	result, err := ParsePortMappings("  8080 : 80 , 8443 : 443  ")
	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestParsePortMappings_TrailingComma(t *testing.T) {
	result, err := ParsePortMappings("8080:80,")
	require.NoError(t, err)
	assert.Len(t, result, 1)
}
