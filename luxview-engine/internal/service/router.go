package service

import (
	"context"
	"fmt"

	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/pkg/logger"
)

// TraefikConfig represents the dynamic Traefik configuration.
type TraefikConfig struct {
	HTTP *TraefikHTTP `json:"http,omitempty"`
}

type TraefikHTTP struct {
	Routers  map[string]TraefikRouter  `json:"routers,omitempty"`
	Services map[string]TraefikService `json:"services,omitempty"`
}

type TraefikRouter struct {
	Rule    string     `json:"rule"`
	Service string     `json:"service"`
	TLS     *TraefikTLS `json:"tls,omitempty"`
}

type TraefikTLS struct {
	CertResolver string `json:"certResolver,omitempty"`
}

type TraefikService struct {
	LoadBalancer TraefikLB `json:"loadBalancer"`
}

type TraefikLB struct {
	Servers []TraefikServer `json:"servers"`
}

type TraefikServer struct {
	URL string `json:"url"`
}

// RouterService generates Traefik dynamic configuration.
type RouterService struct {
	appRepo *repository.AppRepo
	domain  string
}

func NewRouterService(appRepo *repository.AppRepo, domain string) *RouterService {
	return &RouterService{appRepo: appRepo, domain: domain}
}

// GenerateConfig builds the Traefik dynamic config from all running apps.
func (rs *RouterService) GenerateConfig(ctx context.Context) (*TraefikConfig, error) {
	log := logger.With("router")

	apps, err := rs.appRepo.ListAllRunning(ctx)
	if err != nil {
		return nil, fmt.Errorf("list running apps: %w", err)
	}

	if len(apps) == 0 {
		log.Debug().Msg("no running apps, returning empty config")
		return &TraefikConfig{}, nil
	}

	config := &TraefikConfig{
		HTTP: &TraefikHTTP{
			Routers:  make(map[string]TraefikRouter),
			Services: make(map[string]TraefikService),
		},
	}

	for _, app := range apps {
		if app.AssignedPort == 0 {
			continue
		}

		routerName := fmt.Sprintf("app-%s", app.Subdomain)
		serviceName := fmt.Sprintf("app-%s", app.Subdomain)

		// Apps in maintenance, building, or deploying show the maintenance page
		if app.Status == model.AppStatusMaintenance || app.Status == model.AppStatusBuilding || app.Status == "deploying" {
			config.HTTP.Routers[routerName] = TraefikRouter{
				Rule:    fmt.Sprintf("Host(`%s.%s`)", app.Subdomain, rs.domain),
				Service: "maintenance-svc",
				TLS:     &TraefikTLS{CertResolver: "letsencrypt"},
			}
			// Custom domain maintenance route
			if app.CustomDomain != nil && *app.CustomDomain != "" {
				customRouterName := fmt.Sprintf("app-%s-custom", app.Subdomain)
				config.HTTP.Routers[customRouterName] = TraefikRouter{
					Rule:    fmt.Sprintf("Host(`%s`)", *app.CustomDomain),
					Service: "maintenance-svc",
					TLS:     &TraefikTLS{CertResolver: "letsencrypt"},
				}
			}
			continue
		}

		if app.Status != model.AppStatusRunning {
			continue
		}

		config.HTTP.Routers[routerName] = TraefikRouter{
			Rule:    fmt.Sprintf("Host(`%s.%s`)", app.Subdomain, rs.domain),
			Service: serviceName,
			TLS:     &TraefikTLS{CertResolver: "letsencrypt"},
		}

		// Custom domain route
		if app.CustomDomain != nil && *app.CustomDomain != "" {
			customRouterName := fmt.Sprintf("app-%s-custom", app.Subdomain)
			config.HTTP.Routers[customRouterName] = TraefikRouter{
				Rule:    fmt.Sprintf("Host(`%s`)", *app.CustomDomain),
				Service: serviceName,
				TLS:     &TraefikTLS{CertResolver: "letsencrypt"},
			}
		}

		config.HTTP.Services[serviceName] = TraefikService{
			LoadBalancer: TraefikLB{
				Servers: []TraefikServer{
					{URL: fmt.Sprintf("http://host.docker.internal:%d", app.AssignedPort)},
				},
			},
		}
	}

	// Add maintenance service — points to dashboard which serves app-maintenance.html
	if len(config.HTTP.Routers) > 0 {
		config.HTTP.Services["maintenance-svc"] = TraefikService{
			LoadBalancer: TraefikLB{
				Servers: []TraefikServer{
					{URL: "http://luxview-dashboard:80"},
				},
			},
		}
	}

	log.Debug().Int("routes", len(config.HTTP.Routers)).Msg("generated traefik config")
	return config, nil
}
