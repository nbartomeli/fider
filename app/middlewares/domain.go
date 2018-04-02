package middlewares

import (
	"net/http"
	"net/url"

	"github.com/getfider/fider/app/models"

	"github.com/getfider/fider/app"
	"github.com/getfider/fider/app/pkg/env"
	"github.com/getfider/fider/app/pkg/errors"
	"github.com/getfider/fider/app/pkg/web"
)

// Tenant adds either SingleTenant or MultiTenant to the pipeline
func Tenant() web.MiddlewareFunc {
	if env.IsSingleHostMode() {
		return SingleTenant()
	}
	return MultiTenant()
}

// SingleTenant inject default tenant into current context
func SingleTenant() web.MiddlewareFunc {
	return func(next web.HandlerFunc) web.HandlerFunc {
		return func(c web.Context) error {
			tenant, err := c.Services().Tenants.First()
			if err != nil {
				if errors.Cause(err) == app.ErrNotFound {
					return c.Redirect(http.StatusTemporaryRedirect, "/signup")
				}
				return c.Failure(err)
			}

			c.SetTenant(tenant)
			return next(c)
		}
	}
}

// MultiTenant extract tenant information from hostname and inject it into current context
func MultiTenant() web.MiddlewareFunc {
	return func(next web.HandlerFunc) web.HandlerFunc {
		return func(c web.Context) error {
			hostname := stripPort(c.Request.Host)

			// If no tenant is specified, redirect user to getfider.com
			// This is only vadid for fider.io hosting
			if (env.IsProduction() && hostname == "fider.io") ||
				(env.IsDevelopment() && hostname == "dev.fider.io") {
				return c.Redirect(http.StatusTemporaryRedirect, "https://getfider.com")
			}

			tenant, err := c.Services().Tenants.GetByDomain(hostname)
			if err == nil {
				c.SetTenant(tenant)
				return next(c)
			}

			if errors.Cause(err) == app.ErrNotFound {
				c.Logger().Debugf("Tenant not found for '%s'.", hostname)
				return c.NotFound()
			}

			return c.Failure(err)
		}
	}
}

// OnlyActiveTenants blocks requests for inactive tenants
func OnlyActiveTenants() web.MiddlewareFunc {
	return func(next web.HandlerFunc) web.HandlerFunc {
		return func(c web.Context) error {
			if c.Tenant().Status == models.TenantActive {
				return next(c)
			}
			return c.NotFound()
		}
	}
}

// HostChecker checks for a specific host
func HostChecker(baseURL string) web.MiddlewareFunc {
	return func(next web.HandlerFunc) web.HandlerFunc {
		return func(c web.Context) error {
			u, err := url.Parse("http://" + baseURL)
			if err != nil {
				return c.Failure(err)
			}

			if c.Request.Host != u.Host {
				c.Logger().Errorf("%s is not valid for this operation. Only %s is allowed.", c.Request.Host, u.Host)
				return c.NoContent(http.StatusBadRequest)
			}

			return next(c)
		}
	}
}
