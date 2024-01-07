package main

import (
	"embed"
	"fmt"
	"log"

	srvConfig "github.com/CHESSComputing/golib/config"
	mongo "github.com/CHESSComputing/golib/mongo"
	server "github.com/CHESSComputing/golib/server"
	utils "github.com/CHESSComputing/golib/utils"
	"github.com/gin-gonic/gin"
)

// content is our static web server content.
//
//go:embed static
var StaticFs embed.FS

// Verbose defines verbosity level
var Verbose int

// global variables
var _beamlines []string
var _smgr SchemaManager

// helper function to setup our router
func setupRouter() *gin.Engine {
	routes := []server.Route{
		server.Route{Method: "GET", Path: "/:did", Handler: RecordHandler, Authorized: true},
		server.Route{Method: "PUT", Path: "/", Handler: DataHandler, Authorized: true, Scope: "write"},
		server.Route{Method: "POST", Path: "/", Handler: DataHandler, Authorized: true, Scope: "write"},
		server.Route{Method: "POST", Path: "/search", Handler: QueryHandler, Authorized: true},
		server.Route{Method: "POST", Path: "/count", Handler: QueryCountHandler, Authorized: true},
		server.Route{Method: "DELETE", Path: "/:did", Handler: DeleteHandler, Authorized: true, Scope: "write"},
	}
	r := server.Router(routes, nil, "static", srvConfig.Config.CHESSMetaData.WebServer)
	// assign middleware
	r.Use(server.CounterMiddleware())
	return r
}

// Server defines our HTTP server
func Server() {
	// init MongoDB
	log.Println("init mongo", srvConfig.Config.CHESSMetaData.MongoDB.DBUri)
	mongo.InitMongoDB(srvConfig.Config.CHESSMetaData.MongoDB.DBUri)

	// init Verbose
	Verbose = srvConfig.Config.CHESSMetaData.WebServer.Verbose

	// initialize schema manager
	_smgr = SchemaManager{}
	for _, fname := range srvConfig.Config.CHESSMetaData.SchemaFiles {
		_, err := _smgr.Load(fname)
		if err != nil {
			log.Fatalf("unable to load %s error %v", fname, err)
		}
		_beamlines = append(_beamlines, utils.FileName(fname))
	}

	log.Println("Schema", _smgr.String())
	// setup web router and start the service
	r := setupRouter()
	sport := fmt.Sprintf(":%d", srvConfig.Config.CHESSMetaData.WebServer.Port)
	log.Printf("Start HTTP server %s", sport)
	r.Run(sport)
}
