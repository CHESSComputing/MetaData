package main

import (
	"fmt"
	"log"

	authz "github.com/CHESSComputing/common/authz"
	srvConfig "github.com/CHESSComputing/common/config"
	srvMongo "github.com/CHESSComputing/common/mongo"
	"github.com/gin-gonic/gin"
)

func setupRouter() *gin.Engine {
	// Disable Console Color
	// gin.DisableConsoleColor()
	r := gin.Default()

	// GET routes
	r.GET("/meta", MetaHandler)
	r.GET("/meta/record/:mid", MetaRecordHandler)
	r.GET("/meta/:site", MetaSiteHandler)

	// all POST methods ahould be authorized
	authorized := r.Group("/")
	authorized.Use(authz.TokenMiddleware(srvConfig.Config.Authz.ClientId, srvConfig.Config.MetaData.Verbose))
	{
		authorized.POST("/meta", MetaPostHandler)
		authorized.DELETE("/meta/:mid", MetaDeleteHandler)
	}

	return r
}

func Server() {
	// init MongoDB
	srvMongo.InitMongoDB(srvConfig.Config.MetaData.DBUri)

	// setup web router and start the service
	r := setupRouter()
	sport := fmt.Sprintf(":%d", srvConfig.Config.MetaData.WebServer.Port)
	log.Printf("Start HTTP server %s", sport)
	r.Run(sport)
}
