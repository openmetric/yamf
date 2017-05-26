package main

import (
	"fmt"
	"gopkg.in/gin-gonic/gin.v1"
	"io/ioutil"
	"strconv"
)

// ResponseBody of all api requests
type ResponseBody struct {
	Success bool    `json:"success"`
	Message string  `json:"message"`
	Rules   []*Rule `json:"rules"`
}

func WriteResponse(c *gin.Context, code int, success bool, message string, rules []*Rule) {
	c.JSON(code, ResponseBody{
		Success: success,
		Message: message,
		Rules:   rules,
	})
}

func WriteResponseWithNoRule(c *gin.Context, code int, success bool, messageFmt string, v ...interface{}) {
	c.JSON(code, ResponseBody{
		Success: success,
		Message: fmt.Sprintf(messageFmt, v...),
		Rules:   make([]*Rule, 0),
	})
}

func APICreateRule(c *gin.Context) {
	var body []byte
	var rule *Rule
	var err error

	if body, err = ioutil.ReadAll(c.Request.Body); err != nil {
		WriteResponseWithNoRule(c, 500, false, "Error reading request body, err: %s", err)
		return
	}

	if rule, err = NewRuleFromJSON(body, true); err != nil {
		WriteResponseWithNoRule(c, 400, false, "Error parsing body, err: %s", err)
		return
	}

	if _, err = ruleDB.InsertRule(rule); err != nil {
		WriteResponseWithNoRule(c, 500, false, "Error saving rule to db, err: %s", err)
		return
	}

	schedulerManager.AddRule(rule)

	WriteResponse(c, 200, true, "Rule created successfully", []*Rule{rule})
}

func APIListRules(c *gin.Context) {
	var rules []*Rule
	var err error

	if rules, err = ruleDB.GetAllRules(); err != nil {
		WriteResponseWithNoRule(c, 500, false, "Error loading rules from db, err: %s", err)
	} else {
		WriteResponse(c, 200, true, "", rules)
	}
}

func APIGetRule(c *gin.Context) {
	var rule *Rule
	var id int
	var err error

	if id, err = strconv.Atoi(c.Param("id")); err != nil {
		WriteResponseWithNoRule(c, 400, false, "Bad rule id: %s", c.Param("id"))
		return
	}

	if rule, err = ruleDB.GetRule(id); err != nil {
		WriteResponseWithNoRule(c, 500, false, "Error loading rule from db, err: %s", err)
	} else if rule == nil {
		WriteResponseWithNoRule(c, 404, false, "Rule not found")
	} else {
		WriteResponse(c, 200, true, "", []*Rule{rule})
	}
}

func APIUpdateRule(c *gin.Context) {
	var body []byte
	var rule *Rule
	var id int
	var err error

	if id, err = strconv.Atoi(c.Param("id")); err != nil {
		WriteResponseWithNoRule(c, 400, false, "Bad rule id: %s", c.Param("id"))
		return
	}

	if rule, err = ruleDB.GetRule(id); err != nil {
		WriteResponseWithNoRule(c, 500, false, "Error loading old rule from db, err: %s", err)
		return
	} else if rule == nil {
		WriteResponseWithNoRule(c, 404, false, "Rule id does not exist, not updating anything")
		return
	}

	if body, err = ioutil.ReadAll(c.Request.Body); err != nil {
		WriteResponseWithNoRule(c, 500, false, "Error reading request body, err: %s", err)
		return
	}

	if rule, err = NewRuleFromJSON(body, true); err != nil {
		WriteResponseWithNoRule(c, 400, false, "Error parsing body, err: %s", err)
		return
	}

	if err = ruleDB.UpdateRule(id, rule); err != nil {
		WriteResponseWithNoRule(c, 500, false, "Error saving rule to db, err: %s", err)
		return
	}

	schedulerManager.UpdateRule(rule)

	WriteResponse(c, 200, true, "", []*Rule{rule})
}

func APIDeleteRule(c *gin.Context) {
	var err error
	var rule *Rule
	var id int

	if id, err = strconv.Atoi(c.Param("id")); err != nil {
		WriteResponseWithNoRule(c, 400, false, "Bad rule id: %s", c.Param("id"))
		return
	}

	if rule, err = ruleDB.GetRule(id); err != nil {
		WriteResponseWithNoRule(c, 500, false, "Error loading old rule from db, err: %s", err)
		return
	} else if rule == nil {
		WriteResponseWithNoRule(c, 404, false, "Rule id does not exist, not updating anything")
		return
	}

	if err = ruleDB.DeleteRule(id); err != nil {
		WriteResponseWithNoRule(c, 500, false, "Error deleting rule from db, err: %s", err)
		return
	}

	schedulerManager.RemoveRule(id)

	WriteResponse(c, 200, true, "", []*Rule{rule})
}

func RunAPIServer(listenAddr string) {
	router := gin.New()
	v1 := router.Group("v1")

	v1.GET("/rules", APIListRules)
	v1.POST("/rules", APICreateRule)
	v1.GET("/rules/:id", APIGetRule)
	v1.PUT("/rules/:id", APIUpdateRule)
	v1.DELETE("/rules/:id", APIDeleteRule)

	router.NoRoute(func(c *gin.Context) {
		WriteResponseWithNoRule(c, 404, false, "no such endpoint")
	})

	router.Run(listenAddr)
}
