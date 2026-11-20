package bootstrap

import (
	"net/http"
	"testing"

	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/config"

	"github.com/stretchr/testify/assert"
)

const testDeliveryOrderID = "019a0000-0000-7000-8000-000000000001"

var (
	deliveryRead     = struct{ method, path string }{http.MethodGet, "/api/deliveries/orders"}
	deliveryHistory  = struct{ method, path string }{http.MethodGet, "/api/deliveries/orders/" + testDeliveryOrderID + "/history"}
	deliveryCouriers = struct{ method, path string }{http.MethodGet, "/api/deliveries/couriers"}
	deliveryMine     = struct{ method, path string }{http.MethodGet, "/api/deliveries/mine"}
	deliveryDispatch = struct{ method, path string }{http.MethodPost, "/api/deliveries/orders/" + testDeliveryOrderID + "/dispatch"}
	deliveryDeliver  = struct{ method, path string }{http.MethodPost, "/api/deliveries/orders/" + testDeliveryOrderID + "/deliver"}
	deliveryFail     = struct{ method, path string }{http.MethodPost, "/api/deliveries/orders/" + testDeliveryOrderID + "/fail"}
)

func deliveryRoutes() []struct{ method, path string } {
	return []struct{ method, path string }{
		deliveryRead, deliveryHistory, deliveryCouriers, deliveryMine, deliveryDispatch, deliveryDeliver, deliveryFail,
	}
}

func TestRegisterDeliveryRoutes_AbsentOnTheEdge(t *testing.T) {
	jwtService := service.NewJWTService("test-secret", tenant.AnyOrganization())
	manager := tokenFor(t, jwtService, permissions.RoleManager)
	for _, r := range deliveryRoutes() {
		assert.Equal(t, http.StatusNotFound, serve(t, config.ModeEdge, jwtService, manager, r.method, r.path, "{}"), "%s %s", r.method, r.path)
	}
}

func TestRegisterDeliveryRoutes_PresentInTheCloud(t *testing.T) {
	jwtService := service.NewJWTService("test-secret", tenant.AnyOrganization())
	for _, r := range deliveryRoutes() {
		assert.Equal(t, http.StatusUnauthorized, serve(t, config.ModeCloud, jwtService, "", r.method, r.path, "{}"), "%s %s", r.method, r.path)
	}
}

// A route the role passes reaches the service behind it: a 400 from its validation, or a 500 from
// the nil service that Recovery catches. Anything but the guards' 401, 403 and 404.
func TestDeliveryRoutes_PermissionsPerRole(t *testing.T) {
	jwtService := service.NewJWTService("test-secret", tenant.AnyOrganization())
	const passes, refused = http.StatusInternalServerError, http.StatusForbidden
	staff := map[struct{ method, path string }]int{
		deliveryRead: passes, deliveryHistory: passes, deliveryCouriers: passes, deliveryMine: refused,
		deliveryDispatch: passes, deliveryDeliver: passes, deliveryFail: passes,
	}
	tests := []struct {
		name string
		role int
		want map[struct{ method, path string }]int
	}{
		{name: "admin", role: permissions.RoleAdmin, want: map[struct{ method, path string }]int{
			deliveryRead: passes, deliveryHistory: passes, deliveryCouriers: passes, deliveryMine: passes,
			deliveryDispatch: passes, deliveryDeliver: passes, deliveryFail: passes,
		}},
		{name: "manager", role: permissions.RoleManager, want: staff},
		{name: "courier", role: permissions.RoleCourier, want: map[struct{ method, path string }]int{
			deliveryRead: refused, deliveryHistory: refused, deliveryCouriers: refused, deliveryMine: passes,
			deliveryDispatch: passes, deliveryDeliver: passes, deliveryFail: passes,
		}},
	}
	for _, role := range []struct {
		name string
		id   int
	}{{"server", permissions.RoleServer}, {"cooker", permissions.RoleCooker}, {"accountant", permissions.RoleAccountant}, {"agent", permissions.RoleAgent}} {
		none := map[struct{ method, path string }]int{}
		for _, r := range deliveryRoutes() {
			none[r] = refused
		}
		tests = append(tests, struct {
			name string
			role int
			want map[struct{ method, path string }]int
		}{name: role.name, role: role.id, want: none})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := tokenFor(t, jwtService, tt.role)
			for _, r := range deliveryRoutes() {
				got := serve(t, config.ModeCloud, jwtService, token, r.method, r.path, "{}")
				if tt.want[r] == refused {
					assert.Equal(t, refused, got, "%s %s %s", tt.name, r.method, r.path)
				} else {
					assert.NotContains(t, []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound}, got, "%s %s %s", tt.name, r.method, r.path)
				}
			}
		})
	}
}
