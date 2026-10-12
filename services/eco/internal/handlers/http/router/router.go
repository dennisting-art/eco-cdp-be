package router

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/eco/docs"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/eco/internal/auth"
	controller "gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/eco/internal/handlers/http/controller"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/eco/internal/platform/contextkeys"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/eco/internal/platform/response"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/eco/internal/selfpilot"
)

const CustomerIntelligenceViewPermission = "cdphub.cust_intel.view"

type Authenticator interface {
	Authenticate(context.Context, string) (auth.Identity, error)
}
type Pinger interface{ Ping(context.Context) error }

type Dependencies struct {
	Auth     Authenticator
	Cards    *controller.CardsHandler
	Universe *controller.UniverseHandler
	DB       Pinger
	Pilot    *selfpilot.Service
}

type route struct {
	method, path, permission string
	handler                  gin.HandlerFunc
}

func New(d Dependencies) (*gin.Engine, error) {
	if d.Auth == nil || d.Cards == nil || d.Universe == nil || d.DB == nil {
		return nil, errors.New("missing router dependency")
	}
	r := gin.New()
	r.Use(gin.Recovery(), requestContext())
	r.GET("/openapi.yaml", func(c *gin.Context) { c.Data(http.StatusOK, "application/yaml", docs.OpenAPI) })
	r.GET("/docs/openapi.yaml", func(c *gin.Context) { c.Data(http.StatusOK, "application/yaml", docs.OpenAPI) })
	r.GET("/docs/swagger-ui/", func(c *gin.Context) { c.Data(http.StatusOK, "text/html; charset=utf-8", docs.SwaggerUI) })
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	r.GET("/readyz", func(c *gin.Context) {
		if err := d.DB.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	// Every protected route has an explicit operation permission. Empty permissions fail registration.
	routes := []route{
		{http.MethodPost, "/api/v0/customer-intelligence/cards", CustomerIntelligenceViewPermission, d.Cards.QueryCards},
		{http.MethodPost, "/api/v0/customer-intelligence/universe", CustomerIntelligenceViewPermission, d.Universe.QueryUniverse},
	}
	for _, item := range routes {
		if item.permission == "" {
			return nil, errors.New("protected route missing permission")
		}
		r.Handle(item.method, item.path, protected(d.Auth, d.Pilot, item.permission), item.handler)
	}
	r.GET("/api/v0/internal/access", authenticate(d.Auth, d.Pilot), func(c *gin.Context) {
		response.SuccessWithContext(c.Request.Context(), c.Writer, gin.H{"allowed": true})
	})
	if d.Pilot != nil {
		issuer := &pilotIssuer{service: d.Pilot}
		r.POST("/api/v0/internal/self-pilot/token", issuer.issue)
	}
	return r, nil
}

func requestContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		id := c.GetHeader("X-Trace-ID")
		if id == "" || len(id) > 128 {
			var bytes [16]byte
			_, _ = rand.Read(bytes[:])
			id = hex.EncodeToString(bytes[:])
		}
		ctx, cancel := context.WithTimeout(context.WithValue(c.Request.Context(), contextkeys.TraceIdKey, id), 10*time.Second)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Header("X-Trace-ID", id)
		c.Next()
		slog.Info("request", "method", c.Request.Method, "path", c.FullPath(), "status", c.Writer.Status(), "duration_ms", time.Since(start).Milliseconds(), "trace_id", id)
	}
}

func authenticate(authorizer Authenticator, pilot *selfpilot.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := auth.Bearer(c.GetHeader("Authorization"))
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "errors": gin.H{"code": "401", "message": "unauthorized"}})
			c.Abort()
			return
		}
		var identity auth.Identity
		var err error
		if pilot != nil && selfpilot.HasIssuer(token, pilot.Issuer()) {
			var claims selfpilot.Claims
			claims, err = pilot.Validate(token)
			if err != nil {
				err = auth.ErrInvalidToken
			}
			if err == nil {
				identity = auth.Identity{Subject: claims.UserID, Permissions: claims.Permissions}
			}
		} else {
			identity, err = authorizer.Authenticate(c.Request.Context(), token)
		}
		if err != nil {
			status := http.StatusServiceUnavailable
			if errors.Is(err, auth.ErrInvalidToken) {
				status = http.StatusUnauthorized
			}
			c.JSON(status, gin.H{"success": false, "errors": gin.H{"code": http.StatusText(status), "message": strings.ToLower(http.StatusText(status))}})
			c.Abort()
			return
		}
		ctx := context.WithValue(c.Request.Context(), contextkeys.PermissionsKey, identity.Permissions)
		c.Request = c.Request.WithContext(ctx)
		c.Set("identity", identity)
	}
}

func protected(authorizer Authenticator, pilot *selfpilot.Service, permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authenticate(authorizer, pilot)(c)
		if c.IsAborted() {
			return
		}
		identity := c.MustGet("identity").(auth.Identity)
		if !auth.Allows(identity, permission) {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "errors": gin.H{"code": "40007", "message": "access denied"}})
			c.Abort()
		}
	}
}

type pilotIssuer struct {
	service *selfpilot.Service
	mu      sync.Mutex
	window  time.Time
	count   int
}

func (p *pilotIssuer) issue(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	p.mu.Lock()
	if time.Since(p.window) >= time.Minute {
		p.window = time.Now()
		p.count = 0
	}
	p.count++
	limited := p.count > 30
	p.mu.Unlock()
	if limited {
		c.JSON(http.StatusTooManyRequests, gin.H{"success": false})
		return
	}
	var body map[string]json.RawMessage
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 4096))
	err := decoder.Decode(&body)
	if err != nil && err != io.EOF || len(body) != 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false})
		return
	}
	if err == nil && decoder.Decode(new(any)) != io.EOF {
		c.JSON(http.StatusBadRequest, gin.H{"success": false})
		return
	}
	token, claims, err := p.service.Issue(selfpilot.Profile, 0)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false})
		return
	}
	response.SuccessWithContext(c.Request.Context(), c.Writer, gin.H{"access_token": token, "token_type": "Bearer", "expires_in": claims.ExpiresAt - claims.IssuedAt, "profile": claims.Profile})
}
