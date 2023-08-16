package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"

	gql "github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"

	"github.com/joho/godotenv"

	"github.com/gin-gonic/gin"
)

func init() {
	// Log as JSON instead of the default ASCII formatter.
	//log.SetFormatter(&log.JSONFormatter{})

	log.SetFormatter(&log.TextFormatter{
		DisableColors: false,
		FullTimestamp: false,
	})

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)

	// Only log the warning severity or above.
	log.SetLevel(log.TraceLevel)
}

func main() {

	godotenv.Load()

	apiKey := os.Getenv("APOLLO_KEY")
	graphRef := os.Getenv("APOLLO_GRAPH_REF")
	graphRefParts := strings.Split(graphRef, "@")

	log.Debug("Uplink ref is: ", graphRef)

	if len(graphRefParts) != 2 {
		log.Errorf("Could not decode graph ref: %s", graphRef)
		//http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	localSchema := ""
	dryRun := false

	flag.StringVar(&localSchema, "schema", "", "Load local schema instead of remote.")
	flag.BoolVar(&dryRun, "dry", false, "Dry run route creation.")
	flag.Parse()

	schemaSDL := ""

	if localSchema == "" {

		supergraphResult, err := downloadSupergraph(graphRefParts[0], graphRefParts[1], apiKey)
		_ = supergraphResult
		if err != nil {
			log.Errorf("Cannot download supergraph from Apollo: %s", err)
			os.Exit(1)
		}
		schemaSDL = supergraphResult.Data.Service.SchemaTag.CompositionResult.SupergraphSDL
	} else {
		fileContents, err := os.ReadFile(localSchema)
		if err != nil {
			log.Errorf("Cannot read local schema file: %s", err)
			os.Exit(1)
		}
		schemaSDL = string(fileContents)
	}

	// parse schema from Uplink or disk
	input := ast.Source{
		Name:    localSchema,
		Input:   schemaSDL,
		BuiltIn: false,
	}

	ast, err := gql.LoadSchema(&input)
	if err != nil {
		fmt.Printf("Load schema error: %s\n", err)
		os.Exit(1)
	}

	router := gin.Default()
	routeMap, _ := CreateRouteMap(ast)

	for path := range routeMap {
		router.GET(path, getHandler(routeMap))
	}

	for _, thing := range ast.Mutation.Fields {
		log.Infof("%s - POST /%s", thing.Name, ToSnakeCase(thing.Name))
	}

	if !dryRun {
		router.Run()
	}

}
