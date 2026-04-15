// Package middleware provides before/after hooks and response transforms for MCP tool calls.
package middleware

import (
	"context"
)

// Result wraps a tool call result for transformation.
type Result struct {
	Data    any
	IsError bool
	Text    string
}

// BeforeFunc runs before a tool call. Return an error to abort the call.
type BeforeFunc func(ctx context.Context, tool string, params map[string]any) error

// AfterFunc runs after a tool call completes.
type AfterFunc func(ctx context.Context, tool string, result *Result, err error)

// TransformFunc modifies the result before returning to the client.
type TransformFunc func(ctx context.Context, tool string, result *Result) *Result

// Middleware is a single middleware entry.
type Middleware struct {
	before    BeforeFunc
	after     AfterFunc
	transform TransformFunc
}

// Before creates a middleware that runs before every tool call.
func Before(fn BeforeFunc) Middleware {
	return Middleware{before: fn}
}

// After creates a middleware that runs after every tool call.
func After(fn AfterFunc) Middleware {
	return Middleware{after: fn}
}

// Transform creates a middleware that transforms tool results.
func Transform(fn TransformFunc) Middleware {
	return Middleware{transform: fn}
}

// Chain manages an ordered list of middleware.
type Chain struct {
	middlewares []Middleware
}

// Use appends middleware to the chain.
func (c *Chain) Use(mw ...Middleware) {
	c.middlewares = append(c.middlewares, mw...)
}

// RunBefore executes all before hooks in order. Returns the first error, if any.
func (c *Chain) RunBefore(ctx context.Context, tool string, params map[string]any) error {
	for _, mw := range c.middlewares {
		if mw.before != nil {
			if err := mw.before(ctx, tool, params); err != nil {
				return err
			}
		}
	}
	return nil
}

// RunAfter executes all after hooks in order.
func (c *Chain) RunAfter(ctx context.Context, tool string, result *Result, err error) {
	for _, mw := range c.middlewares {
		if mw.after != nil {
			mw.after(ctx, tool, result, err)
		}
	}
}

// RunTransform applies all transform hooks in order.
func (c *Chain) RunTransform(ctx context.Context, tool string, result *Result) *Result {
	for _, mw := range c.middlewares {
		if mw.transform != nil {
			result = mw.transform(ctx, tool, result)
		}
	}
	return result
}

// Len returns the number of middleware in the chain.
func (c *Chain) Len() int {
	return len(c.middlewares)
}
