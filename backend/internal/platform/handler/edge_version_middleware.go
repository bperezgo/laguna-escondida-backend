package handler

import (
	"net/http"

	"laguna-escondida/backend/internal/platform/buildinfo"

	"github.com/gin-gonic/gin"
)

// EdgeVersionHeader carries the release version an edge node was built from.
const EdgeVersionHeader = "X-Edge-Version"

// EdgeVersionMiddleware answers 426 Upgrade Required to an edge below minVersion. An empty
// floor, a missing header (a box that predates it) and an unparsable version such as "dev"
// are accepted, so boxes keep syncing until the floor is first raised.
func EdgeVersionMiddleware(minVersion string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if minVersion != "" && buildinfo.VersionLess(c.GetHeader(EdgeVersionHeader), minVersion) {
			c.JSON(http.StatusUpgradeRequired, gin.H{
				"error":       "edge version is below the minimum supported",
				"min_version": minVersion,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
