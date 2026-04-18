package apps

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/NubeDev/bizzy/pkg/models"
	openapi2mcp "github.com/NubeIO/openapi-mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (f *MCPFactory) registerOpenAPITools(srv *mcp.Server, app *App, install models.AppInstall) {
	var sd *specData

	if app.OpenAPIRemote != nil {
		// Resolve the remote URL by substituting {{key}} placeholders from settings.
		resolved := f.resolveRemoteSpec(app, install)
		if resolved == nil {
			return
		}
		sd = resolved
	} else {
		cached, ok := f.specCache[app.Name]
		if !ok {
			return
		}
		sd = cached
	}

	// Determine base URL from user's settings.
	baseURL := ""
	for _, def := range app.Settings {
		if def.Type == "url" {
			baseURL = install.GetSetting(def.Key)
			break
		}
	}

	// Fallback to spec's servers.
	if baseURL == "" && len(sd.doc.Servers) > 0 {
		baseURL = sd.doc.Servers[0].URL
	}
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	// Build a request handler that injects the user's auth token.
	token := ""
	for _, def := range app.Settings {
		if def.Type == "secret" {
			token = install.GetSetting(def.Key)
			break
		}
	}

	opts := &openapi2mcp.ToolGenOptions{
		NameFormat: func(name string) string {
			return app.Name + "." + name
		},
		TagFilter: app.openAPIIncludeTags(),
		RequestHandler: func(req *http.Request) (*http.Response, error) {
			// Override the base URL.
			if baseURL != "" {
				origPath := req.URL.Path
				newURL := strings.TrimRight(baseURL, "/") + origPath
				if req.URL.RawQuery != "" {
					newURL += "?" + req.URL.RawQuery
				}
				parsed, err := http.NewRequest(req.Method, newURL, req.Body)
				if err != nil {
					return nil, fmt.Errorf("rebuild request: %w", err)
				}
				parsed.Header = req.Header
				req = parsed
			}
			// Inject auth.
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			return http.DefaultClient.Do(req)
		},
	}

	openapi2mcp.RegisterOpenAPITools(srv, sd.ops, sd.doc, opts)
}

// resolveRemoteSpec fetches a remote OpenAPI spec, using a cache with TTL.
func (f *MCPFactory) resolveRemoteSpec(app *App, install models.AppInstall) *specData {
	// Resolve {{key}} placeholders in the URL from user settings.
	url := app.OpenAPIRemote.URL
	for _, def := range app.Settings {
		val := install.GetSetting(def.Key)
		url = strings.ReplaceAll(url, "{{"+def.Key+"}}", val)
	}
	if url == "" {
		return nil
	}

	cacheKey := app.Name + "|" + url

	// Check cache.
	f.remoteCacheMu.RLock()
	entry, ok := f.remoteCache[cacheKey]
	f.remoteCacheMu.RUnlock()
	if ok && time.Since(entry.fetchedAt) < remoteSpecTTL {
		return entry.data
	}

	// Fetch the spec.
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("[mcpfactory] %s: failed to fetch remote openapi from %s: %v", app.Name, url, err)
		if entry != nil {
			return entry.data // serve stale on error
		}
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[mcpfactory] %s: remote openapi returned %d from %s", app.Name, resp.StatusCode, url)
		if entry != nil {
			return entry.data
		}
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[mcpfactory] %s: failed to read remote openapi body: %v", app.Name, err)
		if entry != nil {
			return entry.data
		}
		return nil
	}

	doc, err := openapi2mcp.LoadOpenAPISpecFromBytesLenient(body)
	if err != nil {
		log.Printf("[mcpfactory] %s: failed to parse remote openapi: %v", app.Name, err)
		if entry != nil {
			return entry.data
		}
		return nil
	}

	ops := openapi2mcp.ExtractOpenAPIOperations(doc)

	sd := &specData{doc: doc, ops: ops}
	f.remoteCacheMu.Lock()
	f.remoteCache[cacheKey] = &remoteSpecEntry{data: sd, fetchedAt: time.Now()}
	f.remoteCacheMu.Unlock()

	log.Printf("[mcpfactory] %s: fetched remote openapi from %s (%d operations)", app.Name, url, len(ops))
	return sd
}

// openAPIIncludeTags returns the tag filter from OpenAPIRemote config, or nil.
func (app *App) openAPIIncludeTags() []string {
	if app.OpenAPIRemote == nil {
		return nil
	}
	return app.OpenAPIRemote.IncludeTags
}
