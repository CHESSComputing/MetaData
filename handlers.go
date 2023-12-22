package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"

	beamlines "github.com/CHESSComputing/golib/beamlines"
	srvConfig "github.com/CHESSComputing/golib/config"
	mongo "github.com/CHESSComputing/golib/mongo"
	server "github.com/CHESSComputing/golib/server"
	services "github.com/CHESSComputing/golib/services"
	"github.com/gin-gonic/gin"
	bson "go.mongodb.org/mongo-driver/bson"
)

// MetaParams represents URI storage params in /did end-point
type MetaParams struct {
	DID int64 `uri:"did" binding:"required"`
}

// ApisHandler provides all server routes
func ApisHandler(c *gin.Context) {
	server.ApisHandler(c, _routes)
}

// RecordHandler handles POST queries
func RecordHandler(c *gin.Context) {
	var params MetaParams
	err := c.ShouldBindUri(&params)
	if err != nil {
		rec := services.Response("MetaData", http.StatusBadRequest, services.BindError, err)
		c.JSON(http.StatusBadRequest, rec)
		return
	}
	var records []mongo.Record
	spec := bson.M{"did": params.DID}
	records = mongo.Get(srvConfig.Config.CHESSMetaData.DBName, srvConfig.Config.CHESSMetaData.DBColl, spec, 0, -1)
	if Verbose > 0 {
		log.Println("RecordHandler", spec, records)
	}
	c.JSON(http.StatusOK, records)
}

// helper function to parse incoming HTTP request into ServiceRequest structure
func parseQueryRequest(c *gin.Context) (services.ServiceRequest, error) {
	var rec services.ServiceRequest
	defer c.Request.Body.Close()
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		return rec, err
	}
	err = json.Unmarshal(body, &rec)
	if err != nil {
		return rec, err
	}
	if Verbose > 0 {
		log.Printf("QueryHandler received request %+s", rec.String())
	}
	return rec, nil
}

// helper function to parse input HTTP request JSON data
func parseRequest(c *gin.Context) (services.MetaRecord, error) {
	var rec services.MetaRecord
	defer c.Request.Body.Close()
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		return rec, err
	}
	err = json.Unmarshal(body, &rec)
	if err != nil {
		return rec, err
	}
	if Verbose > 0 {
		log.Printf("QueryHandler received request %+v", rec)
	}
	return rec, nil
}

// DataHandler handles POST upload of meta-data record
func DataHandler(c *gin.Context) {
	rec, err := parseRequest(c)
	if err != nil {
		rec := services.Response("MetaData", http.StatusInternalServerError, services.ParseError, err)
		c.JSON(http.StatusInternalServerError, rec)
		return
	}
	sname := rec.Schema
	if sname == "" {
		err := errors.New("No schema found in meta-data record")
		rec := services.Response("MetaData", http.StatusBadRequest, services.SchemaError, err)
		log.Println("### error", rec.JsonString())
		c.JSON(http.StatusBadRequest, rec)
		return
	}
	schema := beamlines.SchemaFileName(sname)
	record := rec.Record
	if Verbose > 0 {
		log.Printf("insert schema=%s record=%+v", schema, record)
	}
	// insert record to meta-data database
	err = insertData(schema, record)
	if err != nil {
		rec := services.Response("MetaData", http.StatusInternalServerError, services.InsertError, err)
		c.JSON(http.StatusInternalServerError, rec)
		return
	}
	c.JSON(http.StatusOK, services.Response("MetaData", http.StatusOK, services.OK, nil))
}

// QueryHandler handles POST queries
func QueryHandler(c *gin.Context) {

	rec, err := parseQueryRequest(c)
	if err != nil {
		rec := services.Response("MetaData", http.StatusInternalServerError, services.ParseError, err)
		c.JSON(http.StatusInternalServerError, rec)
		return
	}

	// get all attributes we need
	query := rec.ServiceQuery.Query
	idx := rec.ServiceQuery.Idx
	limit := rec.ServiceQuery.Limit

	spec, err := ParseQuery(query)
	if Verbose > 0 {
		log.Printf("search query='%s' spec=%+v", query, spec)
	}
	if err != nil {
		rec := services.Response("MetaData", http.StatusInternalServerError, services.ParseError, err)
		c.JSON(http.StatusInternalServerError, rec)
		return
	}

	var records []mongo.Record
	nrecords := 0
	if spec != nil {
		nrecords = mongo.Count(srvConfig.Config.CHESSMetaData.DBName, srvConfig.Config.CHESSMetaData.DBColl, spec)
		records = mongo.Get(srvConfig.Config.CHESSMetaData.DBName, srvConfig.Config.CHESSMetaData.DBColl, spec, idx, limit)
	}
	if Verbose > 0 {
		log.Printf("spec %v nrecords %d return idx=%d limit=%d", spec, nrecords, idx, limit)
	}
	r := services.Response("MetaData", http.StatusOK, services.OK, nil)
	r.ServiceQuery = services.ServiceQuery{Query: query, Spec: spec, Idx: idx, Limit: limit}
	r.Results = services.ServiceResults{NRecords: nrecords, Records: records}
	c.JSON(http.StatusOK, r)
}

// DeleteHandler handles POST queries
func DeleteHandler(c *gin.Context) {
	err := errors.New("Not implemented yet")
	rec := services.Response("MetaData", http.StatusInternalServerError, services.ParseError, err)
	c.JSON(http.StatusInternalServerError, rec)
}
