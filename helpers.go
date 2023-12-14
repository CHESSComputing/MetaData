package main

import (
	"fmt"

	server "github.com/CHESSComputing/common/server"
	utils "github.com/CHESSComputing/common/utils"
	"github.com/gin-gonic/gin"
)

func pagination(c *gin.Context, query string, nres, startIdx, limit int) string {
	tmpl := server.MakeTmpl(c, "Search")
	url := fmt.Sprintf("/search?query=%s", query)
	if nres > 0 {
		tmpl["StartIndex"] = fmt.Sprintf("%d", startIdx+1)
	} else {
		tmpl["StartIndex"] = fmt.Sprintf("%d", startIdx)
	}
	if nres > startIdx+limit {
		tmpl["EndIndex"] = fmt.Sprintf("%d", startIdx+limit)
	} else {
		tmpl["EndIndex"] = fmt.Sprintf("%d", nres)
	}
	tmpl["Total"] = fmt.Sprintf("%d", nres)
	tmpl["FirstUrl"] = makeURL(url, "first", startIdx, limit, nres)
	tmpl["PrevUrl"] = makeURL(url, "prev", startIdx, limit, nres)
	tmpl["NextUrl"] = makeURL(url, "next", startIdx, limit, nres)
	tmpl["LastUrl"] = makeURL(url, "last", startIdx, limit, nres)
	page := utils.TmplPage(StaticFs, "pagiantion.tmpl", tmpl)
	return fmt.Sprintf("%s<br>", page)
}

func makeURL(url, urlType string, startIdx, limit, nres int) string {
	var out string
	var idx int
	if urlType == "first" {
		idx = 0
	} else if urlType == "prev" {
		if startIdx != 0 {
			idx = startIdx - limit
		} else {
			idx = 0
		}
	} else if urlType == "next" {
		idx = startIdx + limit
	} else if urlType == "last" {
		j := 0
		for i := 0; i < nres; i = i + limit {
			if i > nres {
				break
			}
			j = i
		}
		idx = j
	}
	out = fmt.Sprintf("%s&amp;idx=%d&&amp;limit=%d", url, idx, limit)
	return out
}
