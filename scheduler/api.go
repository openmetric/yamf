package scheduler

import (
	"encoding/json"
	"fmt"
	"github.com/HouzuoGuo/tiedot/dberr"
	"github.com/braintree/manners"
	"github.com/openmetric/yamf/internal/types"
	"gopkg.in/gin-gonic/gin.v1"
	"io/ioutil"
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

func (s *Scheduler) apiGetRule(c *gin.Context) {
	var rule *types.Rule
	var id int
	var err error

	if id, err = strconv.Atoi(c.Param("id")); err != nil {
		apiWriteFail(c, 400, "Bad rule id: %s", c.Param("id"))
		return
	}

	if rule, err = s.rdb.Get(id); dberr.Type(err) == dberr.ErrorNoDoc {
		apiWriteFail(c, 404, "Rule not found")
	} else if err != nil {
		apiWriteFail(c, 500, "Error loading rule from db, err: %s", err)
	} else {
		apiWriteSuccess(c, []*types.Rule{rule})
	}
}

func (s *Scheduler) apiListRules(c *gin.Context) {
	var rules []*types.Rule
	var errors []error
	var err error

	if rules, errors, err = s.rdb.GetAll(); err != nil {
		apiWriteFail(c, 500, "Error loading rules from db, err: %s", err)
	} else {
		var result []*types.Rule
		for i, rule := range rules {
			if errors[i] == nil {
				result = append(result, rule)
			}
		}
		apiWriteSuccess(c, result)
	}
}

func (s *Scheduler) apiCreateRule(c *gin.Context) {
	var body []byte
	var err error

	if body, err = ioutil.ReadAll(c.Request.Body); err != nil {
		apiWriteFail(c, 500, "Error reading request body, err: %s", err)
		return
	}

	rule := &types.Rule{}
	if err = json.Unmarshal(body, rule); err != nil {
		apiWriteFail(c, 400, "Error parsing body, err: %s", err)
		return
	}
	// force reset rule.ID to 0, user should not provide an ID
	rule.ID = 0
	if err = rule.Validate(); err != nil {
		apiWriteFail(c, 400, "Invalid rule: %s", err)
		return
	}

	if _, err = s.rdb.Insert(rule); err != nil {
		apiWriteFail(c, 500, "Error saving rule to db, err: %s", err)
		return
	}

	s.schedule(rule)

	apiWriteSuccess(c, []*types.Rule{rule})
}

func (s *Scheduler) apiUpdateRule(c *gin.Context) {
	var body []byte
	var rule *types.Rule
	var err error
	var id int

	if id, err = strconv.Atoi(c.Param("id")); err != nil {
		apiWriteFail(c, 400, "Bad rule id: %s", c.Param("id"))
		return
	}

	if rule, err = s.rdb.Get(id); err != nil {
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

	if err = s.rdb.Update(id, rule); err != nil {
		apiWriteFail(c, 500, "Error saving rule to db, err: %s", err)
		return
	}

	s.schedule(rule)

	apiWriteSuccess(c, []*types.Rule{rule})
}

func (s *Scheduler) apiDeleteRule(c *gin.Context) {
	var err error
	var rule *types.Rule
	var id int

	if id, err = strconv.Atoi(c.Param("id")); err != nil {
		apiWriteFail(c, 400, "Bad rule id: %s", c.Param("id"))
		return
	}

	if rule, err = s.rdb.Get(id); err != nil {
		apiWriteFail(c, 500, "Error loading rule from db, err: %s", err)
		return
	} else if rule == nil {
		apiWriteFail(c, 404, "Rule id does not exist, not updating anything")
		return
	}

	if err = s.rdb.Delete(id); err != nil {
		apiWriteFail(c, 500, "Error deleting rule from db, err: %s", err)
	}

	s.stop(id)

	apiWriteSuccess(c, []*types.Rule{rule})
}

func (s *Scheduler) runAPIServer() {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())
	router.NoRoute(func(c *gin.Context) { apiWriteFail(c, 404, "no such endpoint") })

	v1 := router.Group("v1")
	v1.GET("/rules", s.apiListRules)
	v1.POST("/rules", s.apiCreateRule)
	v1.GET("/rules/:id", s.apiGetRule)
	v1.PUT("/rules/:id", s.apiUpdateRule)
	v1.PATCH("/rules/:id", s.apiUpdateRule)
	v1.DELETE("/rules/:id", s.apiDeleteRule)

	go func() {
		s.apiServerStop = make(chan struct{})
		manners.ListenAndServe(s.config.ListenAddress, router)
		close(s.apiServerStop)
	}()
}

func (s *Scheduler) stopAPIServer() {
	s.logger.Infof("shutting down api server...")
	manners.Close()
	<-s.apiServerStop
}
