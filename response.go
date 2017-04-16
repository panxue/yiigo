package yiigo

import "github.com/gin-gonic/gin"

/**
 * API返回JSON数据
 * @param c *gin.Context
 * @param code int 返回的 Code
 * @param msg string 返回的 Message
 * @param data ...interface{} 返回的数据
 */
func ReturnJson(c *gin.Context, code int, msg string, data ...interface{}) {
	obj := gin.H{
		"code": code,
		"msg":  msg,
	}

	if len(data) > 0 {
		obj["data"] = data[0]
	}

	c.JSON(200, obj)
}