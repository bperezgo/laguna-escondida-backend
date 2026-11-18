package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"laguna-escondida/backend/internal/domain/permissions"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRequirePermission_Success(t *testing.T) {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role_ids", []int{permissions.RoleAdmin})
		c.Next()
	})
	router.GET("/test", RequirePermission(permissions.OrdersRead), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequirePermission_Forbidden(t *testing.T) {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role_ids", []int{permissions.RoleCooker})
		c.Next()
	})
	router.GET("/test", RequirePermission(permissions.ExpensesCreate), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Insufficient permissions", response["error"])
	assert.Equal(t, "expenses:create", response["required"])
}

func TestRequirePermission_NoRoleIDs(t *testing.T) {
	router := gin.New()
	router.GET("/test", RequirePermission(permissions.OrdersRead), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Authentication required", response["error"])
}

func TestRequirePermission_InvalidRoleData(t *testing.T) {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role_ids", "invalid")
		c.Next()
	})
	router.GET("/test", RequirePermission(permissions.OrdersRead), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Invalid role data", response["error"])
}

func TestRequireAnyPermission_HasFirstPermission(t *testing.T) {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role_ids", []int{permissions.RoleServer})
		c.Next()
	})
	router.GET("/test", RequireAnyPermission(permissions.OrdersRead, permissions.ExpensesRead), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireAnyPermission_HasSecondPermission(t *testing.T) {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role_ids", []int{permissions.RoleAccountant})
		c.Next()
	})
	router.GET("/test", RequireAnyPermission(permissions.OrdersCreate, permissions.ExpensesRead), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireAnyPermission_NoMatchingPermission(t *testing.T) {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role_ids", []int{permissions.RoleCooker})
		c.Next()
	})
	router.GET("/test", RequireAnyPermission(permissions.ExpensesCreate, permissions.SuppliersCreate), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequireAnyPermission_NoRoleIDs(t *testing.T) {
	router := gin.New()
	router.GET("/test", RequireAnyPermission(permissions.OrdersRead), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireAnyPermission_InvalidRoleData(t *testing.T) {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role_ids", "invalid")
		c.Next()
	})
	router.GET("/test", RequireAnyPermission(permissions.OrdersRead), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRequirePermission_MultipleRoles(t *testing.T) {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role_ids", []int{permissions.RoleServer, permissions.RoleAccountant})
		c.Next()
	})
	router.GET("/test", RequirePermission(permissions.ExpensesRead), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
