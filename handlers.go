package main

import (
	"encoding/hex"
	"fmt"

	srvConfig "github.com/CHESSComputing/common/config"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SiteParams represents URI storage params in /meta/:site end-point
type SiteParams struct {
	Site string `uri:"site" binding:"required"`
}

// MetaIdParams represents URI storage params in /meta/:mid end-point
type MetaIdParams struct {
	ID string `uri:"mid" binding:"required"`
}

// MetaHandler provives access to GET /meta end-point
func MetaHandler(c *gin.Context) {
	data := metadata("")
	c.JSON(200, gin.H{"status": "ok", "data": data})
}

// MetaSiteHandler provides access to GET /meta/:site end-point
func MetaSiteHandler(c *gin.Context) {
	var params SiteParams
	if err := c.ShouldBindUri(&params); err == nil {
		data := metadata(params.Site)
		c.JSON(200, gin.H{"status": "ok", "data": data})
	} else {
		c.JSON(400, gin.H{"status": "fail", "error": err.Error()})
	}
}

// MetaRecordHandler provides access to GET /meta/:site end-point
func MetaRecordHandler(c *gin.Context) {
	var params MetaIdParams
	if err := c.ShouldBindUri(&params); err == nil {
		data := getRecord(params.ID)
		c.JSON(200, gin.H{"status": "ok", "data": data})
	} else {
		c.JSON(400, gin.H{"status": "fail", "error": err.Error()})
	}
}

// MetaPostHandler provides access to POST /meta end-point
func MetaPostHandler(c *gin.Context) {
	var meta MetaData
	err := c.BindJSON(&meta)
	if err == nil {
		if meta.ID == "" {
			if uuid, err := uuid.NewRandom(); err == nil {
				meta.ID = hex.EncodeToString(uuid[:])
			}
		}
		_metaData = append(_metaData, meta)
		// upsert into MongoDB
		if srvConfig.Config.MetaData.MongoDB.DBUri != "" {
			//         meta.mongoUpsert("ID")
			meta.mongoInsert()
		}
		c.JSON(200, gin.H{"status": "ok"})
	} else {
		c.JSON(400, gin.H{"status": "fail", "error": err.Error()})
	}
}

// MetaDeleteHandler provides access to Delete /meta/:mid end-point
func MetaDeleteHandler(c *gin.Context) {
	var params MetaIdParams
	if err := c.ShouldBindUri(&params); err == nil {
		var metaData []MetaData
		for _, meta := range _metaData {
			if meta.ID != params.ID {
				metaData = append(metaData, meta)
				// remove record from MongoDB
				meta.mongoRemove()
			}
		}
		if len(_metaData) == len(metaData) {
			// record was not found
			msg := fmt.Sprintf("record %s was not found in MetaData service", params.ID)
			c.JSON(400, gin.H{"status": "fail", "error": msg})
			return
		}
		_metaData = metaData
		c.JSON(200, gin.H{"status": "ok"})
	} else {
		c.JSON(400, gin.H{"status": "fail", "error": err.Error()})
	}
}
