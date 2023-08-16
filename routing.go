package main

import (
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/vektah/gqlparser/v2/ast"
)

// CreateRouteMap - build query details for all operations reachable
// by REST routes.
func CreateRouteMap(ast *ast.Schema) (map[string]*GetMethod, error) {

	routeMap := make(map[string]*GetMethod, 10)

	for _, thing := range ast.Query.Fields {
		if strings.HasPrefix(thing.Name, "__") {
			continue
		}
		sigs, _ := CreateGetMethod(thing.Name, "", "Query", nil, ast)

		for _, sig := range sigs {
			log.Infof("GET %s - %#v", sig.Path, sig.FieldPath)

			for k, v := range sig.QueryString {
				log.Infof("  %s=%s", k, v.Type)
			}

			routeMap[sig.Path] = sig

		}

	}

	return routeMap, nil
}
