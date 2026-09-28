package main

import (
	"os"

	"showmethestory/internal/httpapi"
)

func runWebApplication(resources *appResources) error {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	} else {
		port = ":" + port
	}
	httpapi.StartWebServer(resources.apiCfg, resources.apiCfgPath, resources.logger, port, resources.progDir, version, resources.staticFS)
	return nil
}
