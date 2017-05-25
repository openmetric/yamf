package main

import (
	"fmt"
	"github.com/HouzuoGuo/tiedot/dberr"
	"gopkg.in/gin-gonic/gin.v1"
	"io/ioutil"
	"strconv"
)

// ResponseBody of all api requests
type ResponseBody struct {
	Success      bool    `json:"success"`
	ErrorMessage string  `json:"error_message"`
	Rules        []*Rule `json:"rules"`
}

func APICreateRule(c *gin.Context) {
	var body []byte
	var err error
	var rule *Rule

	body, err = ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(500, ResponseBody{
			Success:      false,
			ErrorMessage: fmt.Sprint("Error reading request body, err: ", err),
		})
	}

	if rule, err = NewRuleFromJSON(body); err != nil {
		c.JSON(400, ResponseBody{
			Success:      false,
			ErrorMessage: fmt.Sprint("Error parsing body, err: ", err),
		})
		return
	}

	if err = store.SaveRule(rule); err != nil {
		c.JSON(500, ResponseBody{
			Success:      false,
			ErrorMessage: fmt.Sprint("Error saving rule to db, err: ", err),
		})
		return
	}

	// TODO start scheduling the rule

	c.JSON(200, ResponseBody{
		Success: true,
		Rules:   []*Rule{rule},
	})
}

func APIUpdateRule(c *gin.Context) {
	var err error
	var id int
	var rule *Rule
	var body []byte

	if id, err = strconv.Atoi(c.Param("id")); err != nil {
		c.JSON(400, ResponseBody{
			Success:      false,
			ErrorMessage: fmt.Sprint("Bad rule id: ", c.Param("id")),
			Rules:        nil,
		})
		return
	}

	if rule, err = store.LoadRule(id); err != nil {
		// this should not happen, however, we should still check
		c.JSON(500, ResponseBody{
			Success:      false,
			ErrorMessage: fmt.Sprint("Error querying old rule, err: ", err),
			Rules:        nil,
		})
		return
	} else if rule == nil {
		c.JSON(404, ResponseBody{
			Success:      false,
			ErrorMessage: "Rule id does not exist, not update anything",
			Rules:        nil,
		})
		return
	}

	body, err = ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(500, ResponseBody{
			Success:      false,
			ErrorMessage: fmt.Sprint("Error reading request body, err: ", err),
		})
	}

	if rule, err = NewRuleFromJSON(body); err != nil {
		c.JSON(400, ResponseBody{
			Success:      false,
			ErrorMessage: fmt.Sprint("Error parsing body, err: ", err),
		})
		return
	}

	rule.ID = id
	if err = store.SaveRule(rule); err != nil {
		c.JSON(500, ResponseBody{
			Success:      false,
			ErrorMessage: fmt.Sprint("Error saving rule to db, err: ", err),
		})
		return
	}

	// TODO update scheduling

	c.JSON(200, ResponseBody{
		Success: true,
		Rules:   []*Rule{rule},
	})
}

func APIGetRule(c *gin.Context) {
	var err error
	var id int
	var rule *Rule

	if id, err = strconv.Atoi(c.Param("id")); err != nil {
		c.JSON(400, ResponseBody{
			Success:      false,
			ErrorMessage: fmt.Sprint("Bad rule id: ", c.Param("id")),
			Rules:        nil,
		})
		return
	}

	if rule, err = store.LoadRule(id); err != nil {
		c.JSON(500, ResponseBody{
			Success:      false,
			ErrorMessage: fmt.Sprint("Error loading rule, err: ", err),
			Rules:        nil,
		})
		return
	} else if rule == nil {
		c.JSON(404, ResponseBody{
			Success:      false,
			ErrorMessage: "Rule not found",
			Rules:        nil,
		})
		return
	} else {
		c.JSON(200, ResponseBody{
			Success:      true,
			ErrorMessage: "",
			Rules:        []*Rule{rule},
		})
		return
	}
}

func APIListRules(c *gin.Context) {
	var rules []*Rule
	var err error

	if rules, err = store.LoadRules(); err != nil {
		c.JSON(500, ResponseBody{
			Success:      false,
			ErrorMessage: fmt.Sprint("Error loading rules, err: ", err),
			Rules:        nil,
		})
		return
	} else {
		c.JSON(200, ResponseBody{
			Success:      true,
			ErrorMessage: "",
			Rules:        rules,
		})
		return
	}
}

func APIDeleteRule(c *gin.Context) {
	var err error
	var id int

	if id, err = strconv.Atoi(c.Param("id")); err != nil {
		c.JSON(400, ResponseBody{
			Success:      false,
			ErrorMessage: fmt.Sprint("Bad rule id: ", c.Param("id")),
			Rules:        nil,
		})
		return
	}

	// TODO stop scheduling

	err = store.DeleteRule(id)

	if dberr.Type(err) == dberr.ErrorNoDoc {
		c.JSON(404, ResponseBody{
			Success:      false,
			ErrorMessage: "Rule not found",
			Rules:        nil,
		})
		return
	}

	if err != nil {
		c.JSON(500, ResponseBody{
			Success:      false,
			ErrorMessage: fmt.Sprint("Error deleting rule, err: ", err),
			Rules:        nil,
		})
		return
	}

	c.JSON(200, ResponseBody{
		Success:      true,
		ErrorMessage: "",
		Rules:        nil,
	})
}

func RunAPIServer(listenAddr string) {
	router := gin.Default()

	router.GET("/v1/rules", APIListRules)
	router.POST("/v1/rules", APICreateRule)
	router.GET("/v1/rules/:id", APIGetRule)
	router.PUT("/v1/rules/:id", APIUpdateRule)
	router.DELETE("/v1/rules/:id", APIDeleteRule)

	router.NoRoute(func(c *gin.Context) {
		c.JSON(404, ResponseBody{
			Success:      false,
			ErrorMessage: "no such endpoint",
		})
	})

	router.Run(listenAddr)
}
