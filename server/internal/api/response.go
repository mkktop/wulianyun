package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Resp struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Resp{Code: 0, Msg: "ok", Data: data})
}

func Fail(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, Resp{Code: code, Msg: msg})
}

// PageData 分页响应
type PageData struct {
	Total int64       `json:"total"`
	List  interface{} `json:"list"`
}
