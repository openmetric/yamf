package scheduler

import (
	"encoding/json"
	"fmt"
	"github.com/openmetric/yamf/internal/types"
	"gopkg.in/gin-gonic/gin.v1"
	"io/ioutil"
	"os"
	"strconv"
)

type apiResponseBody struct {
	Success bool          `json:"success"`
	Message string        `json:"message"`
	Rules   []*types.Rule `json:"rules"`
}

func apiWriteSuccess(c *gin.Context, rules []*types.Rule) {
	c.JSON(200, apiResponseBody{
		Success: true,
		Message: "",
		Rules:   rules,
	})
}

func apiWriteFail(c *gin.Context, code int, messageFmt string, v ...interface{}) {
	c.JSON(code, apiResponseBody{
		Success: false,
		Message: fmt.Sprintf(messageFmt, v...),
		Rules:   make([]*types.Rule, 0),
	})
}

func (w *worker) apiGetRule(c *gin.Context) {
	var rule *types.Rule
	var id int
	var err error

	if id, err = strconv.Atoi(c.Param("id")); err != nil {
		apiWriteFail(c, 400, "Bad rule id: %s", c.Param("id"))
		return
	}

	if rule, err = w.rdb.Get(id); err != nil {
		apiWriteFail(c, 500, "Error loading rule from db, err: %s", err)
	} else if rule == nil {
		apiWriteFail(c, 404, "Rule not found")
	} else {
		apiWriteSuccess(c, []*types.Rule{rule})
	}
}

func (w *worker) apiListRules(c *gin.Context) {
	var rules []*types.Rule
	var err error

	if rules, err = w.rdb.GetAll(); err != nil {
		apiWriteFail(c, 500, "Error loading rules from db, err: %s", err)
	} else {
		apiWriteSuccess(c, rules)
	}
}

func (w *worker) apiCreateRule(c *gin.Context) {
	var body []byte
	var err error

	if body, err = ioutil.ReadAll(c.Request.Body); err != nil {
		apiWriteFail(c, 500, "Error reading request body, err: %s", err)
		return
	}

	rule := &types.Rule{}
	if err = json.Unmarshal(body, rule); err != nil {
		apiWriteFail(c, 400, "Error parsing body, err: %s", err)
	}
	// force reset rule.ID to 0, user should not provide an ID
	rule.ID = 0
	if err = rule.Validate(); err != nil {
		apiWriteFail(c, 400, "Invalid rule: %s", err)
	}

	if _, err = w.rdb.Insert(rule); err != nil {
		apiWriteFail(c, 500, "Error saving rule to db, err: %s", err)
		return
	}

	w.startSchedule(rule)

	apiWriteSuccess(c, []*types.Rule{rule})
}

func (w *worker) apiUpdateRule(c *gin.Context) {
	var body []byte
	var rule *types.Rule
	var err error
	var id int

	if id, err = strconv.Atoi(c.Param("id")); err != nil {
		apiWriteFail(c, 400, "Bad rule id: %s", c.Param("id"))
		return
	}

	if rule, err = w.rdb.Get(id); err != nil {
		apiWriteFail(c, 500, "Error loading old rule from db, err: %s", err)
		return
	} else if rule == nil {
		apiWriteFail(c, 404, "Rule id does not exist, not updating anything")
		return
	}

	if body, err = ioutil.ReadAll(c.Request.Body); err != nil {
		apiWriteFail(c, 500, "Error reading request body, err: %s", err)
		return
	}

	rule = &types.Rule{}
	if err = json.Unmarshal(body, rule); err != nil {
		apiWriteFail(c, 400, "Error parsing body, err: %s", err)
	}
	// force reset rule.ID to 0, user should not provide an ID
	rule.ID = 0
	if err = rule.Validate(); err != nil {
		apiWriteFail(c, 400, "Invalid rule: %s", err)
	}

	if err = w.rdb.Update(id, rule); err != nil {
		apiWriteFail(c, 500, "Error saving rule to db, err: %s", err)
		return
	}

	w.updateSchedule(rule)

	apiWriteSuccess(c, []*types.Rule{rule})
}

func (w *worker) apiDeleteRule(c *gin.Context) {
	var err error
	var rule *types.Rule
	var id int

	if id, err = strconv.Atoi(c.Param("id")); err != nil {
		apiWriteFail(c, 400, "Bad rule id: %s", c.Param("id"))
		return
	}

	if rule, err = w.rdb.Get(id); err != nil {
		apiWriteFail(c, 500, "Error loading rule from db, err: %s", err)
		return
	} else if rule == nil {
		apiWriteFail(c, 404, "Rule id does not exist, not updating anything")
		return
	}

	if err = w.rdb.Delete(id); err != nil {
		apiWriteFail(c, 500, "Error deleting rule from db, err: %s", err)
	}

	w.stopSchedule(id)

	apiWriteSuccess(c, []*types.Rule{rule})
}

func (w *worker) runAPIServer() {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())

	logFile, err := os.OpenFile(w.config.HTTPLogFilename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		fmt.Println("Failed to open logfile:", err)
		os.Exit(1)
	}
	router.Use(gin.LoggerWithWriter(logFile))

	v1 := router.Group("v1")
	v1.GET("/rules", w.apiListRules)
	v1.POST("/rules", w.apiCreateRule)
	v1.GET("/rules/:id", w.apiGetRule)
	v1.PUT("/rules/:id", w.apiUpdateRule)
	v1.PATCH("/rules/:id", w.apiUpdateRule)
	v1.DELETE("/rules/:id", w.apiDeleteRule)

	router.NoRoute(func(c *gin.Context) { apiWriteFail(c, 404, "no such endpoint") })
	router.Run(w.config.ListenAddr) // nolint
}
