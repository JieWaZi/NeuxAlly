package plugininstallation

import (
	"errors"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	"testing"
)

func TestBridgeEntrypointsRequireTrustedVersionBeforeProcessResolution(t *testing.T) {
	service := New(Config{})
	if _, _, err := service.Bridge(models.PluginVersion{}); !errors.Is(err, status_error.MCPConnectorTrustRequired) {
		t.Fatalf("direct bridge bypassed trust: %v", err)
	}
	if _, _, _, err := service.BridgeLease(models.PluginVersion{}); !errors.Is(err, status_error.MCPConnectorTrustRequired) {
		t.Fatalf("leased bridge bypassed trust: %v", err)
	}
}
