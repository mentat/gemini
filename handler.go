package main

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func getHandler(routeMap map[string]*GetMethod) gin.HandlerFunc {

	return func(c *gin.Context) {
		log.Infof("GET - Handler called, route: %s", c.FullPath())

		route := routeMap[c.FullPath()]
		if route == nil {
			log.Errorf("Route %s not found.", c.FullPath())
			c.JSON(404, gin.H{
				"message": "No such route.",
			})
			return
		}
		variables := make(map[string]interface{})
		if route.IDInPath {
			variables["id"] = c.Param("id")
		}
		for k, v := range c.Request.URL.Query() {
			if len(v) > 1 {
				variables[k] = v
			} else {
				variables[k] = v[0]
			}
		}

		queryString, opName := BuildQuery(route, &variables)

		_ = opName

		log.Infof("Route found, building GQL.")
		log.Infof(queryString)

		postBody := make(map[string]interface{})
		postBody["variables"] = variables
		postBody["operationName"] = opName
		postBody["query"] = queryString

		c.JSON(200, postBody)
	}
}

func postHandler(routeMap map[string]*GetMethod) gin.HandlerFunc {

	return func(c *gin.Context) {
		log.Infof("POST - Handler called, route: %s", c.FullPath())

		route := routeMap[c.FullPath()]
		if route == nil {
			log.Errorf("Route %s not found.", c.FullPath())
			c.JSON(404, gin.H{
				"message": "No such route.",
			})
			return
		}

		postData := make(map[string]interface{})

		err := json.NewDecoder(c.Request.Body).Decode(&postData)
		if err != nil {
			log.Errorf("Cannot decode request body: %s", err)
			return
		}

		if route.IDInPath {
			postData["id"] = c.Param("id")
		}

		queryString, opName := BuildQuery(route, &postData)

		log.Infof("Route found, building GQL.")
		log.Infof(queryString)

		postBody := make(map[string]interface{})
		postBody["variables"] = postData
		postBody["operationName"] = opName
		postBody["query"] = queryString

		c.JSON(200, postBody)
	}
}
