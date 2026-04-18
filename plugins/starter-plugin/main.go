package main

import (
	"fmt"
	"log"

	"github.com/NubeDev/bizzy/pkg/pluginsdk"
)

func main() {
	p := pluginsdk.NewPlugin("starter", "0.1.0", "Minimal starter plugin")
	p.SetPreamble("Use plugin.starter.echo to echo text back to the user.")

	schema := pluginsdk.Params("text", "string", "Text to echo", true)
	p.AddTool(pluginsdk.Tool{
		Name:        "echo",
		Description: "Echo text back with plugin metadata",
		Parameters:  schema,
		Handler: func(params map[string]any) (any, error) {
			text, ok := params["text"].(string)
			if !ok || text == "" {
				return nil, fmt.Errorf("missing required parameter: text")
			}

			return map[string]any{
				"plugin":  "starter",
				"tool":    "echo",
				"message": text,
			}, nil
		},
	})

	if err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
