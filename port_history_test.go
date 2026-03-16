package ecsexecpf

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadPortHistory_FileDoesNotExist_ReturnsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")
	history, err := loadPortHistoryFromPath(path)
	require.NoError(t, err)
	assert.Empty(t, history.Services)
}

func TestLoadPortHistory_CorruptJSON_ReturnsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corrupt.json")
	require.NoError(t, os.WriteFile(path, []byte("{invalid"), 0644))

	history, err := loadPortHistoryFromPath(path)
	require.NoError(t, err)
	assert.Empty(t, history.Services)
}

func TestSaveAndLoadPortHistory_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	original := PortHistory{
		Services: map[string][][]PortMapping{
			"my-service": {
				{
					{RemotePort: 80, LocalPort: 8080},
					{RemotePort: 443, LocalPort: 8443},
				},
			},
		},
	}

	require.NoError(t, savePortHistoryToPath(path, original))

	loaded, err := loadPortHistoryFromPath(path)
	require.NoError(t, err)
	assert.Equal(t, original, loaded)
}

func TestAddMapping_NewService(t *testing.T) {
	history := PortHistory{Services: make(map[string][][]PortMapping)}
	mappings := []PortMapping{{RemotePort: 80, LocalPort: 8080}}

	result := AddMapping(history, "svc-a", mappings)

	assert.Len(t, result.Services["svc-a"], 1)
	assert.Equal(t, mappings, result.Services["svc-a"][0])
}

func TestAddMapping_DeduplicatesExisting(t *testing.T) {
	mappings := []PortMapping{{RemotePort: 80, LocalPort: 8080}}
	history := PortHistory{
		Services: map[string][][]PortMapping{
			"svc-a": {mappings},
		},
	}

	result := AddMapping(history, "svc-a", mappings)

	assert.Len(t, result.Services["svc-a"], 1)
	assert.Equal(t, mappings, result.Services["svc-a"][0])
}

func TestAddMapping_PrependsNewMapping(t *testing.T) {
	old := []PortMapping{{RemotePort: 80, LocalPort: 8080}}
	new := []PortMapping{{RemotePort: 443, LocalPort: 8443}}
	history := PortHistory{
		Services: map[string][][]PortMapping{
			"svc-a": {old},
		},
	}

	result := AddMapping(history, "svc-a", new)

	require.Len(t, result.Services["svc-a"], 2)
	assert.Equal(t, new, result.Services["svc-a"][0])
	assert.Equal(t, old, result.Services["svc-a"][1])
}

func TestAddMapping_CapsAtTenEntries(t *testing.T) {
	entries := make([][]PortMapping, 10)
	for i := range entries {
		entries[i] = []PortMapping{{RemotePort: i, LocalPort: i + 1000}}
	}
	history := PortHistory{
		Services: map[string][][]PortMapping{
			"svc-a": entries,
		},
	}

	new := []PortMapping{{RemotePort: 999, LocalPort: 9999}}
	result := AddMapping(history, "svc-a", new)

	assert.Len(t, result.Services["svc-a"], 10)
	assert.Equal(t, new, result.Services["svc-a"][0])
}

func TestAddMapping_DoesNotMutateOriginal(t *testing.T) {
	mappings := []PortMapping{{RemotePort: 80, LocalPort: 8080}}
	history := PortHistory{Services: make(map[string][][]PortMapping)}

	AddMapping(history, "svc-a", mappings)

	assert.Empty(t, history.Services)
}

func TestFormatMappings(t *testing.T) {
	mappings := []PortMapping{
		{RemotePort: 80, LocalPort: 8080},
		{RemotePort: 443, LocalPort: 8443},
	}
	assert.Equal(t, "8080:80, 8443:443", FormatMappings(mappings))
}

func TestPortMappingString(t *testing.T) {
	pm := PortMapping{RemotePort: 80, LocalPort: 8080}
	assert.Equal(t, "8080:80", pm.String())
}
