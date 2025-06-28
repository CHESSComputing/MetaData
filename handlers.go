package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	beamlines "github.com/CHESSComputing/golib/beamlines"
	srvConfig "github.com/CHESSComputing/golib/config"
	lexicon "github.com/CHESSComputing/golib/lexicon"
	ql "github.com/CHESSComputing/golib/ql"
	"github.com/CHESSComputing/golib/server"
	services "github.com/CHESSComputing/golib/services"
	"github.com/gin-gonic/gin"
)

// MetaParams represents /record?did=bla end-point
type MetaParams struct {
	DID string `form:"did"`
}

// MetaDetailsHandler provides MetaData details dictionary via /meta end-point
func MetaDetailsHandler(c *gin.Context) {
	records := _smgr.MetaDetails()
	c.JSON(http.StatusOK, records)
}

// SummaryHandler handles queries via GET requests
func SummaryHandler(c *gin.Context) {
	summary := make(map[string]any)
	var attrs []string
	if err := c.BindJSON(&attrs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return

	}

	// total number of records
	spec := map[string]any{}
	nrec := metaDB.Count(
		srvConfig.Config.CHESSMetaData.DBName,
		srvConfig.Config.CHESSMetaData.DBColl,
		spec)
	summary["total"] = nrec

	// find unique values of attributes
	for _, field := range attrs {
		records, err := metaDB.Distinct(
			srvConfig.Config.CHESSMetaData.DBName,
			srvConfig.Config.CHESSMetaData.DBColl,
			field)
		if err == nil {
			summary[field] = records
		} else {
			log.Printf("ERROR: fail to look up %s, error %v", field, err)
		}
	}
	c.JSON(http.StatusOK, summary)
}

// RecordHandler handles queries via GET requests
func RecordHandler(c *gin.Context) {
	var params MetaParams
	err := c.Bind(&params)
	if err != nil {
		rec := services.Response("MetaData", http.StatusBadRequest, services.BindError, err)
		c.JSON(http.StatusBadRequest, rec)
		return
	}
	var records []map[string]any
	spec := map[string]any{"did": params.DID}
	records = metaDB.Get(
		srvConfig.Config.CHESSMetaData.DBName,
		srvConfig.Config.CHESSMetaData.DBColl,
		spec, 0, -1)
	if Verbose > 0 {
		log.Println("RecordHandler", spec, records)
	}
	c.JSON(http.StatusOK, records)
}

// helper function to parse spec (JSON dict) from HTTP request
func parseSpec(c *gin.Context) (map[string]any, error) {
	var spec map[string]any
	defer c.Request.Body.Close()
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		return spec, err
	}
	err = json.Unmarshal(body, &spec)
	if err != nil {
		return spec, err
	}
	return spec, nil
}

// RecordsHandler handles requests to get set of records for provided meta parametes
func RecordsHandler(c *gin.Context) {
	spec, err := parseSpec(c)
	if err != nil {
		log.Println("ERROR:", err)
		rec := services.Response("MetaData", http.StatusInternalServerError, services.ParseError, err)
		c.JSON(http.StatusInternalServerError, rec)
		return
	}
	var records []map[string]any
	records = metaDB.Get(
		srvConfig.Config.CHESSMetaData.DBName,
		srvConfig.Config.CHESSMetaData.DBColl,
		spec, 0, -1)
	if Verbose > 0 {
		log.Println("RecordsHandler", spec, records)
	}
	accept := c.GetHeader("Accept")
	if accept == "application/x-ndjson" || accept == "application/ndjson" {
		handleNDJSON(c, records)
		return
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
		log.Printf("ERROR: unable to unmarshal response body %s, error %v", string(body), err)
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
		log.Println("ERROR:", err)
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
	err = lexicon.ValidateRecord(record)
	if err != nil {
		log.Println("ERROR:", err)
		rec := services.Response("MetaData", http.StatusInternalServerError, services.ValidateError, err)
		c.JSON(http.StatusInternalServerError, rec)
		return
	}

	// insert record to meta-data database
	attrs := srvConfig.Config.DID.Attributes
	sep := srvConfig.Config.DID.Separator
	div := srvConfig.Config.DID.Divider
	updateRecord := false
	if c.Request.Method == "PUT" {
		updateRecord = true
	}
	did, err := insertData(schema, record, attrs, sep, div, updateRecord)
	if err != nil {
		log.Println("ERROR:", err)
		rec := services.Response("MetaData", http.StatusInternalServerError, services.InsertError, err)
		c.JSON(http.StatusInternalServerError, rec)
		return
	}
	var records []map[string]any
	resp := services.Response("MetaData", http.StatusOK, services.OK, nil)
	r := make(map[string]any)
	r["did"] = did
	records = append(records, r)
	resp.Results = services.ServiceResults{NRecords: 1, Records: records}
	c.JSON(http.StatusOK, resp)
}

// UpdateParams represents JSON struct used by UpdateHandler
type UpdateParams struct {
	Doi    string `json:"doi"`
	Did    string `json:"did"`
	Public bool   `json:"public"`
}

// UpdateDoiHandler handles POST upload of meta-data record
func UpdateDoiHandler(c *gin.Context) {
	// Bind JSON payload to struct
	var rec UpdateParams
	if err := c.ShouldBindJSON(&rec); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err := updateDoiData(rec.Did, rec.Doi, rec.Public)
	if err != nil {
		log.Println("ERROR:", err)
		rec := services.Response("MetaData", http.StatusInternalServerError, services.ParseError, err)
		c.JSON(http.StatusInternalServerError, rec)
		return
	}
}

// QueryCountHandler handles POST queries
func QueryCountHandler(c *gin.Context) {

	rec, err := parseQueryRequest(c)
	if err != nil {
		log.Println("ERROR:", err)
		rec := services.Response("MetaData", http.StatusInternalServerError, services.ParseError, err)
		c.JSON(http.StatusInternalServerError, rec)
		return
	}

	// get all attributes we need
	query := rec.ServiceQuery.Query
	var spec map[string]any
	if query == "" && rec.ServiceQuery.Spec != nil {
		spec = rec.ServiceQuery.Spec
		err = nil
	} else {
		spec, err = ql.ParseQuery(query)
	}
	if Verbose > 0 {
		log.Printf("search query='%s' spec=%+v", query, spec)
	}
	if err != nil {
		log.Println("ERROR:", err)
		rec := services.Response("MetaData", http.StatusInternalServerError, services.ParseError, err)
		c.JSON(http.StatusInternalServerError, rec)
		return
	}

	nrecords := 0
	if spec != nil {
		nrecords = metaDB.Count(srvConfig.Config.CHESSMetaData.DBName, srvConfig.Config.CHESSMetaData.DBColl, spec)
	}
	if Verbose > 0 {
		log.Printf("spec %v nrecords %d", spec, nrecords)
	}
	c.JSON(http.StatusOK, nrecords)
}

// QueryHandler handles POST queries
func QueryHandler(c *gin.Context) {

	rec, err := parseQueryRequest(c)
	if err != nil {
		log.Println("ERROR:", err)
		rec := services.Response("MetaData", http.StatusInternalServerError, services.ParseError, err)
		c.JSON(http.StatusInternalServerError, rec)
		return
	}

	// get all attributes we need
	query := rec.ServiceQuery.Query
	idx := rec.ServiceQuery.Idx
	limit := rec.ServiceQuery.Limit
	sortOrder := rec.ServiceQuery.SortOrder
	sortKeys := rec.ServiceQuery.SortKeys

	spec := rec.ServiceQuery.Spec
	if spec != nil {
		if Verbose > 0 {
			log.Printf("use rec.ServiceQuery.Spec=%+v", spec)
		}
	} else {
		spec, err = ql.ParseQuery(query)
		if Verbose > 0 {
			log.Printf("search query='%s' spec=%+v", query, spec)
		}
		if err != nil {
			log.Println("ERROR:", err)
			rec := services.Response("MetaData", http.StatusInternalServerError, services.ParseError, err)
			c.JSON(http.StatusInternalServerError, rec)
			return
		}
	}

	var records []map[string]any
	nrecords := 0
	if spec != nil {
		nrecords = metaDB.Count(srvConfig.Config.CHESSMetaData.DBName, srvConfig.Config.CHESSMetaData.DBColl, spec)
		if len(sortKeys) > 0 {
			records = metaDB.GetSorted(
				srvConfig.Config.CHESSMetaData.DBName,
				srvConfig.Config.CHESSMetaData.DBColl,
				spec, sortKeys, sortOrder, idx, limit)
		} else {
			records = metaDB.Get(
				srvConfig.Config.CHESSMetaData.DBName,
				srvConfig.Config.CHESSMetaData.DBColl,
				spec, idx, limit)
		}
	}
	if Verbose > 0 {
		log.Printf("spec %v sortedKeys %v nrecords %d return idx=%d limit=%d", spec, sortKeys, nrecords, idx, limit)
	}
	//     r := services.Response("MetaData", http.StatusOK, services.OK, nil)
	//     r.ServiceQuery = services.ServiceQuery{Query: query, Spec: spec, Idx: idx, Limit: limit}
	//     r.Results = services.ServiceResults{NRecords: nrecords, Records: records}
	//     c.JSON(http.StatusOK, r)
	c.JSON(http.StatusOK, records)
}

// DeleteHandler handles POST queries
func DeleteHandler(c *gin.Context) {
	did := c.Request.FormValue("did")
	_, user, err := server.GetAuthTokenUser(c)
	if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "no user found in token", "message": "no user found", "code": services.RemoveError})
				return
	}
	// find user btrs and see if it matches the provided did
	if srvConfig.Config.Frontend.CheckBtrs && srvConfig.Config.Embed.DocDb == "" {
		attrs, err := services.UserAttributes(user)
		if err == nil {
			// check user btrs and return error if user does not have any associations with Btrs
			if len(attrs.Btrs) == 0 {
				msg := fmt.Sprintf("User %s does not associated with any BTRs, delete record is deined", user)
				log.Println("ERROR:", msg)
				c.JSON(http.StatusBadRequest, gin.H{"error": "no btr matches", "message": msg, "code": services.RemoveError})
				return
			}
			btrFound := false
			for _, btr := range attrs.Btrs {
				if strings.Contains(did, btr) {
					btrFound = true
				}
			}
			if !btrFound {
				msg := fmt.Sprintf("User %s BTR does not match did", user)
				log.Println("ERROR:", msg)
				c.JSON(http.StatusBadRequest, gin.H{"error": "no btr matches", "message": msg, "code": services.RemoveError})
				return
			}
		}
	}
	spec := make(map[string]any)
	spec["did"] = did
	err = metaDB.Remove(
		srvConfig.Config.CHESSMetaData.DBName,
		srvConfig.Config.CHESSMetaData.DBColl,
		spec)
	status := http.StatusOK
	srvCode := services.OK
	if err != nil {
		status = http.StatusBadRequest
		srvCode = services.RemoveError
	}
	rec := services.Response("MetaData", status, srvCode, err)
	c.JSON(status, rec)
}
