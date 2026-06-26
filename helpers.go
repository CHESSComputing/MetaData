package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	utils "github.com/CHESSComputing/golib/utils"
	"github.com/gin-gonic/gin"
)

// helper function to extract schema name from schema file name
func schemaName(fname string) string {
	arr := strings.Split(fname, "/")
	return strings.Split(arr[len(arr)-1], ".")[0]
}

// helper function to write data in NDJSON data format
func handleNDJSON(c *gin.Context, records []map[string]any) {
	// Set the Content-Type header to NDJSON
	c.Header("Content-Type", "application/x-ndjson")
	c.Status(http.StatusOK)

	// Use the Gin context's Writer to stream the response
	for _, record := range records {
		// Marshal each record to JSON
		line, err := json.Marshal(record)
		if err != nil {
			// Handle JSON marshalling error
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to serialize record"})
			return
		}
		// Write each JSON line followed by a newline
		_, _ = c.Writer.Write(append(line, '\n'))
	}
}

// helper function to adjust timestamp in spec
func adjustTimestamp(specPtr *map[string]any) {
	spec := *specPtr
	if val, ok := spec["date"]; ok {
		switch ts := val.(type) {
		case string:
			epoch, err := utils.RFC3339ToEpoch(ts)
			if err == nil {
				spec["date"] = epoch
			}
		}
	}
	if Verbose > 2 {
		log.Printf("adjusted spec: %+v", spec)
	}
}

func specRecord(tmplRec map[string]any) map[string]any {
	rec := make(map[string]any)
	// TODO: implement logic of getting spec record from tmpl record
	return rec
}
