package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
)

const SupergraphQuery = `query SupergraphFetchQuery($graph_id: ID!, $variant: String!) {
  frontendUrlRoot
  service(id: $graph_id) {
    variants {
      name
    }
    schemaTag(tag: $variant) {
      compositionResult {
        __typename
        supergraphSdl
        graphCompositionID
      }
    }
    mostRecentCompositionPublish(graphVariant: $variant) {
      errors {
        message
        code
      }
    }
  }
}
`

type GQLQuery struct {
	Variables     map[string]interface{} `json:"variables"`
	Query         string                 `json:"query"`
	OperationName string                 `json:"operationName"`
}

type UplinkRouterConfig struct {
	TypeName      string `json:"__typename"`
	ID            string `json:"id"`
	SupergraphSDL string `json:"supergraphSdl"`
}

type UplinkRouterConfigWrapper struct {
	RouterConfig UplinkRouterConfig `json:"routerConfig"`
}

type UplinkResult struct {
	Data UplinkRouterConfigWrapper `json:"data"`
}

type BuildErrorLocation struct {
	Line   int64 `json:"line"`
	Column int64 `json:"column"`
}

type BuildStatusError struct {
	Message   string
	Locations []BuildErrorLocation
}

type BuildStatusWebhook struct {
	EventType           string             `json:"eventType"`
	EventID             string             `json:"eventID"`
	SupergraphSchemaURL string             `json:"supergraphSchemaURL"`
	BuildSucceeded      bool               `json:"buildSucceeded"`
	BuildErrors         []BuildStatusError `json:"buildErrors"`
	GraphID             string             `json:"graphID"`
	VariantID           string             `json:"variantID"`
	Timestamp           string             `json:"timestamp"`
}

type CompositionResult struct {
	TypeName           string `json:"__typename"`
	SupergraphSDL      string `json:"supergraphSdl"`
	GraphCompositionID string `json:"graphCompositionID"`
}

type SchemaTag struct {
	CompositionResult CompositionResult `json:"compositionResult"`
}

type ServiceResult struct {
	SchemaTag SchemaTag `json:"schemaTag"`
}

type SupergraphFetch struct {
	FrontendURLRoot string        `json:"frontendUrlRoot"`
	Service         ServiceResult `json:"service"`
}

type SupergraphResult struct {
	Data SupergraphFetch `json:"data"`
}

func downloadSupergraph(graphID, variant, apiKey string) (*SupergraphResult, error) {

	var q = GQLQuery{
		Variables: map[string]interface{}{
			"graph_id": graphID,
			"variant":  variant,
		},
		Query:         SupergraphQuery,
		OperationName: "SupergraphFetchQuery",
	}

	body, _ := json.Marshal(q)

	tr := &http.Transport{
		DisableKeepAlives:  true,
		DisableCompression: false,
	}

	httpClient := http.Client{Transport: tr}
	postRequest, err := http.NewRequest(
		"POST",
		"https://graphql.api.apollographql.com/api/graphql",
		bytes.NewBuffer(body))

	if err != nil {
		log.Errorf("Could create request %s", err)
		return nil, fmt.Errorf("could create request %s", err)
	}

	postRequest.Close = true
	postRequest.Header.Set("Accept", "*/*")
	postRequest.Header.Set("Content-Type", "application/json")
	postRequest.Header.Set("X-API-Key", apiKey)
	postRequest.Header.Set("apollographql-client-name", "go-gemini")
	postRequest.Header.Set("apollographql-client-version", "0.1.0")

	resp, err := httpClient.Do(postRequest)

	if err != nil {
		log.Errorf("Could not retrieve supergraph SDL %s", err)
		return nil, fmt.Errorf("could not retrieve supergraph SDL %s", err)
	}
	defer resp.Body.Close()

	supergraphResult := &SupergraphResult{}

	// Decode response
	err = json.NewDecoder(resp.Body).Decode(supergraphResult)
	if err != nil {
		log.Errorf("Could not decode supergraph result: %s", err)
		return nil, fmt.Errorf("could not decode supergraph result: %s", err)
	}
	return supergraphResult, nil
}
