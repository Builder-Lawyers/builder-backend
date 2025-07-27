package main

import "github.com/Builder-Lawyers/builder-backend/builder/cmd"

//go:generate go tool oapi-codegen -config .\api\cfg.models.yaml .\api\openapi.yaml
//go:generate go tool oapi-codegen -config .\api\cfg.templater.yaml ..\templater\api\openapi.yaml
//go:generate go tool oapi-codegen -config .\api\cfg.yaml .\api\openapi.yaml
func main() {
	//build.RunFrontendBuild("./test-task")
	//s3 := storage.NewStorage()
	//s3.ListFiles()
	cmd.Init()
}
