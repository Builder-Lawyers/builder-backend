package main

import "github.com/Builder-Lawyers/builder-backend/templater/cmd"

//go:generate go tool oapi-codegen -config .\api\cfg.models.yaml .\api\openapi.yaml
//go:generate go tool oapi-codegen -config .\api\cfg.builder.yaml ..\builder\api\openapi.yaml
//go:generate go tool oapi-codegen -config .\api\cfg.yaml .\api\openapi.yaml
func main() {
	cmd.Init()
}
