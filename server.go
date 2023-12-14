package main

import (
	"embed"
	"fmt"
	"log"

	authz "github.com/CHESSComputing/common/authz"
	srvConfig "github.com/CHESSComputing/common/config"
	mongo "github.com/CHESSComputing/common/mongo"
	server "github.com/CHESSComputing/common/server"
	utils "github.com/CHESSComputing/common/utils"
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

// helper function to handle base path of URL requests
func base(api string) string {
	b := srvConfig.Config.CHESSMetaData.WebServer.Base
	return utils.BasePath(b, api)
}

func setupRouter() *gin.Engine {
	// Disable Console Color
	// gin.DisableConsoleColor()
	r := gin.Default()

	// GET routes
	r.GET("/meta", MetaHandler)
	r.GET("/meta/record/:mid", MetaRecordHandler)
	r.GET("/meta/:site", MetaSiteHandler)

	// CHESSDataManagement APIs
	r.GET(base("/faq"), server.FAQHandler)
	r.GET(base("/status"), server.StatusHandler)
	r.GET(base("/schemas"), SchemasHandler)
	r.GET(base("/data"), DataHandler)
	r.GET(base("/process"), ProcessHandler)

	// TMP: until I implement tokens in client
	r.GET(base("/search"), SearchHandler)
	r.POST(base("/search"), SearchHandler)
	r.POST(base("/updateRecord"), UpdateRecordHandler)
	r.POST(base("/json"), UploadJsonHandler)
	// end of TMP

	r.GET(base("/"), DataHandler)

	// all POST methods ahould be authorized
	authorized := r.Group("/")
	authorized.Use(authz.TokenMiddleware(srvConfig.Config.Authz.ClientID, Verbose))
	{
		//         authorized.POST(base("/updateRecord"), UpdateRecordHandler)
		//         authorized.POST(base("/json"), UploadJsonHandler)
		//         authorized.POST(base("/search"), SearchHandler)
		//         authorized.GET(base("/search"), SearchHandler)
		authorized.POST("/meta", MetaPostHandler)
		authorized.DELETE("/meta/:mid", MetaDeleteHandler)
	}

	return r
}

func Server() {
	// init MongoDB
	log.Println("init mongo", srvConfig.Config.CHESSMetaData.MongoDB.DBUri)
	mongo.InitMongoDB(srvConfig.Config.CHESSMetaData.MongoDB.DBUri)

	// init Verbose
	Verbose = srvConfig.Config.CHESSMetaData.WebServer.Verbose

	// init server.StaticFs
	server.StaticFs = StaticFs

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
